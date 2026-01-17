package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spf13/viper"
	"github.com/yuin/goldmark"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
)

// ... (existing code structures)

type LeaseTemplateData struct {
	BailleurNom     string
	BailleurAdresse string
	BailleurEmail   string

	LocataireNom   string
	LocataireEmail string

	AdresseLogement string
	Surface         string
	NbPieces        string
	Dependances     string
	ClasseDPE       string
	TypeHabitat     string
	ModeChauffage   string
	EauChaude       string

	DateDebut        string
	DureeBail        string
	LoyerHC          string
	Charges          string
	IsForfaitCharges bool
	TotalMensuel     string
	DepotGarantie    string

	VilleSignature string
	DateSignature  string
}

// GenerateAndSave generates the lease document and persists it to storage.
func (s *LeaseService) GenerateAndSave(ctx context.Context, leaseID int32, userID int32) error {
	content, _, err := s.GenerateLeaseDocument(ctx, leaseID, userID)
	if err != nil {
		return err
	}

	storageName := fmt.Sprintf("lease_%d.html", leaseID)
	if _, err := s.storage.Save(storageName, content); err != nil {
		return fmt.Errorf("failed to save lease document: %w", err)
	}

	// Update DB with the download URL (which serves the stored file)
	downloadURL := fmt.Sprintf("/api/v1/leases/%d/download", leaseID)

	err = s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		return q.UpdateLeaseContractURL(ctx, postgres.UpdateLeaseContractURLParams{
			ID:          leaseID,
			ContractUrl: pgtype.Text{String: downloadURL, Valid: true},
		})
	})

	if err != nil {
		return fmt.Errorf("failed to update lease contract url: %w", err)
	}

	s.logger.Info("lease document generated and saved", zap.Int("lease_id", int(leaseID)), zap.String("url", downloadURL))
	return nil
}

// GetLeaseDocumentContent retrieves the lease document from storage if available,
// otherwise generates it on the fly (legacy/fallback).
func (s *LeaseService) GetLeaseDocumentContent(ctx context.Context, leaseID int32, userID int32) ([]byte, string, error) {
	storageName := fmt.Sprintf("lease_%d.html", leaseID)

	// 1. Try Storage
	// IMPORTANT: Even if we get it from storage, we MUST verify access.
	// Since GenerateLeaseDocument verifies access, calling it is safe.
	// But if we return stored content, we skip that check.
	// We must perform a lightweight access check first if we want to serve stored content.

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		l, err := q.GetLease(ctx, leaseID)
		if err != nil {
			return fmt.Errorf("lease not found: %w", err)
		}
		prop, err := q.GetProperty(ctx, l.PropertyID.Int32)
		if err != nil {
			return fmt.Errorf("property not found: %w", err)
		}

		if l.TenantID.Int32 != userID && prop.OwnerID.Int32 != userID {
			return fmt.Errorf("access denied: user %d is not a party to this lease", userID)
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}

	if s.storage.Exists(storageName) {
		content, err := s.storage.Get(storageName)
		if err == nil {
			return content, "contract.html", nil
		}
		s.logger.Warn("failed to read stored lease", zap.Error(err))
	}

	// 2. Fallback: Generate (this will re-verify access redundant but safe)
	return s.GenerateLeaseDocument(ctx, leaseID, userID)
}

func (s *LeaseService) GenerateLeaseDocument(ctx context.Context, leaseID int32, userID int32) ([]byte, string, error) {
	s.logger.Info("generating lease document", zap.Int("lease_id", int(leaseID)))

	// 1. Fetch Data
	var lease postgres.Lease
	var prop postgres.Property
	var tenant, owner postgres.User

	err := s.txManager.WithTx(ctx, func(q postgres.Querier) error {
		// Get Lease
		l, err := q.GetLease(ctx, leaseID)
		if err != nil {
			return fmt.Errorf("lease not found: %w", err)
		}
		lease = l

		// Get Property
		prop, err = q.GetProperty(ctx, l.PropertyID.Int32)
		if err != nil {
			return fmt.Errorf("property not found: %w", err)
		}

		// Authorization Check
		// If tenant is nil, only owner can view. If tenant is set, check match.
		if l.TenantID.Valid {
			if l.TenantID.Int32 != userID && prop.OwnerID.Int32 != userID {
				return fmt.Errorf("access denied: user %d is not a party to this lease", userID)
			}
		} else {
			// Draft mode: Only owner can view (for now), or invited user if we had a way to verify token context
			if prop.OwnerID.Int32 != userID {
				return fmt.Errorf("access denied: user %d is not the owner of this draft lease", userID)
			}
		}

		// Get Tenant (Real or Draft)
		if l.TenantID.Valid {
			tenant, err = q.GetUserById(ctx, l.TenantID.Int32)
			if err != nil {
				return fmt.Errorf("tenant not found: %w", err)
			}
		} else {
			// Fetch from Invitation
			invitation, err := q.GetInvitationByLeaseID(ctx, pgtype.Int4{Int32: leaseID, Valid: true})
			if err != nil {
				// If no invitation found, use placeholders
				tenant = postgres.User{
					FirstName: pgtype.Text{String: "Locataire", Valid: true},
					LastName:  pgtype.Text{String: "Invité", Valid: true},
					Email:     "email@pending.com",
				}
			} else {
				// We don't have name in invitation, only email.
				// The spec says "Store tenant_info temporarily".
				// IF we want names in draft PDF, we need to store them.
				// For now, use "Invité" or fetch if we stored it in JSON (we didn't yet, only email in Invitation).
				// NOTE: Spec says "tenant_info" in payload. I didn't add JSON column to Invitation.
				// I only added email.
				// LIMITATION: Use "Futur Locataire" + Email from invitation.
				tenant = postgres.User{
					FirstName: pgtype.Text{String: "Futur", Valid: true},
					LastName:  pgtype.Text{String: "Locataire", Valid: true},
					Email:     invitation.TenantEmail,
				}
			}
		}

		// Get Owner
		owner, err = q.GetUserById(ctx, prop.OwnerID.Int32)
		if err != nil {
			return fmt.Errorf("owner not found: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, "", err
	}

	// 2. Select Template
	var templateName string
	if prop.RentalType == postgres.PropertyTypeSeasonal {
		templateName = "template_saisonnier.md"
	} else {
		// Long Term
		if prop.IsFurnished.Bool {
			templateName = "template_bail_meuble.md"
		} else {
			templateName = "template_bail_nu.md"
		}
	}

	assetsDir := viper.GetString("ASSETS_DIR")
	if assetsDir == "" {
		assetsDir = "assets"
	}
	templatePath := filepath.Join(assetsDir, "templates", "leases", templateName)
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read template %s: %w", templateName, err)
	}

	// 3. Prepare Data
	rent, _ := lease.RentAmount.Float64Value()
	deposit, _ := lease.DepositAmount.Float64Value()
	charges, _ := lease.ChargesAmount.Float64Value() // Fix: Use lease charges, not property default

	total := rent.Float64 + charges.Float64

	// Unmarshal Property Details
	var details struct {
		Surface     float64 `json:"surface"`
		RoomCount   int     `json:"room_count"`
		DPE         string  `json:"dpe"`
		HeatingMode string  `json:"heating_mode"`
		HotWater    string  `json:"hot_water"`
	}
	if len(prop.Details) > 0 {
		_ = json.Unmarshal(prop.Details, &details)
	}

	// Default values if missing
	surface := fmt.Sprintf("%.2f m²", details.Surface)
	if details.Surface == 0 {
		surface = "__ m²"
	}
	nbPieces := fmt.Sprintf("%d", details.RoomCount)
	if details.RoomCount == 0 {
		nbPieces = "__"
	}
	dpe := details.DPE
	if dpe == "" {
		dpe = "__"
	}

	data := LeaseTemplateData{
		BailleurNom:     fmt.Sprintf("%s %s", owner.LastName.String, owner.FirstName.String),
		BailleurEmail:   owner.Email,
		BailleurAdresse: "Non renseignée (voir profil)",

		LocataireNom:   fmt.Sprintf("%s %s", tenant.LastName.String, tenant.FirstName.String),
		LocataireEmail: tenant.Email,

		AdresseLogement: prop.Address,

		Surface:       surface,
		NbPieces:      nbPieces,
		Dependances:   "Aucune",
		ClasseDPE:     dpe,
		TypeHabitat:   "Grand Collectif", // Default or fetch from details
		ModeChauffage: details.HeatingMode,
		EauChaude:     details.HotWater,

		DateDebut:        lease.StartDate.Time.Format("02/01/2006"),
		DureeBail:        "1 an (renouvelable)",
		LoyerHC:          fmt.Sprintf("%.2f", rent.Float64),
		Charges:          fmt.Sprintf("%.2f", charges.Float64),
		IsForfaitCharges: prop.IsFurnished.Bool, // Often fixed for furnished
		TotalMensuel:     fmt.Sprintf("%.2f", total),
		DepotGarantie:    fmt.Sprintf("%.2f", deposit.Float64),

		VilleSignature: "SecuLoc (En ligne)",
		DateSignature:  time.Now().Format("02/01/2006"),
	}

	// 4. Execute Template (Fill variables)
	tmpl, err := template.New("lease").Parse(string(content))
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, "", fmt.Errorf("failed to execute template: %w", err)
	}

	// 5. Convert Markdown to HTML (Goldmark)
	var htmlBuf bytes.Buffer
	if err := goldmark.Convert(buf.Bytes(), &htmlBuf); err != nil {
		return nil, "", fmt.Errorf("failed to convert markdown to html: %w", err)
	}

	// 6. Wrap in Styled HTML Container
	finalHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>Contrat de Location</title>
<style>
body { font-family: 'Helvetica', 'Arial', sans-serif; max-width: 800px; margin: 40px auto; padding: 20px; line-height: 1.6; color: #333; }
h1, h2, h3 { color: #000; border-bottom: 2px solid #333; padding-bottom: 10px; margin-top: 30px; }
h1 { font-size: 24px; text-align: center; border: none; text-transform: uppercase; letter-spacing: 2px; }
strong { font-weight: bold; }
ul { margin-bottom: 1em; padding-left: 20px; }
li { margin-bottom: 0.5em; }
p { margin-bottom: 0.8em; text-align: justify; }
.signature-box { margin-top: 50px; display: flex; justify-content: space-between; page-break-inside: avoid; }
.signature-col { width: 45%%; border: 1px solid #ccc; padding: 20px; height: 150px; }
</style>
</head>
<body>
%s
<div class="signature-box">
	<div class="signature-col">
		<strong>Le Bailleur</strong><br>
		%s<br><br>
		<em>(Signé électroniquement)</em>
	</div>
	<div class="signature-col">
		<strong>Le Locataire</strong><br>
		%s
	</div>
</div>
</body>
</html>`, htmlBuf.String(), data.BailleurNom, data.LocataireNom)

	return []byte(finalHTML), "contract.html", nil
}

// GenerateLeasePDF generates a PDF from the lease HTML using basic HTML-to-PDF conversion.
func (s *LeaseService) GenerateLeasePDF(ctx context.Context, leaseID int32, userID int32) ([]byte, string, error) {
	// 1. Get HTML Content
	htmlBytes, _, err := s.GenerateLeaseDocument(ctx, leaseID, userID)
	if err != nil {
		return nil, "", err
	}

	// 2. Setup Rod (Headless Browser)
	// We use a custom launcher to ensure it works in Docker/Dev envs
	l := launcher.New()
	u := l.MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	// 3. Create Page & Set Content
	page := browser.MustPage()

	// Set content safely
	if err := page.SetDocumentContent(string(htmlBytes)); err != nil {
		return nil, "", fmt.Errorf("failed to set page content: %w", err)
	}

	// Wait for network idle to ensure fonts/images loaded
	page.MustWaitLoad()

	// 4. Print to PDF
	// page.PDF() returns a StreamReader in recent versions, or []byte in older ones.
	// Based on the error "cannot use *rod.StreamReader as []byte", it returns a stream.
	pdfStream, err := page.PDF(&proto.PagePrintToPDF{
		PaperWidth:      floatPtr(8.27),
		PaperHeight:     floatPtr(11.69),
		MarginTop:       floatPtr(0.5),
		MarginBottom:    floatPtr(0.5),
		MarginLeft:      floatPtr(0.5),
		MarginRight:     floatPtr(0.5),
		PrintBackground: true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate PDF stream: %w", err)
	}

	// Read the stream to []byte
	pdfBytes, err := io.ReadAll(pdfStream)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read PDF stream: %w", err)
	}

	return pdfBytes, "contract.pdf", nil
}

// Helper to convert float pointer for Rod
func floatPtr(v float64) *float64 { return &v }
