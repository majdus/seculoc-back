package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"seculoc-back/internal/adapter/storage/postgres"
)

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
func (s *LeaseService) GenerateAndSave(ctx context.Context, leaseID int32) error {
	content, _, err := s.GenerateLeaseDocument(ctx, leaseID)
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
func (s *LeaseService) GetLeaseDocumentContent(ctx context.Context, leaseID int32) ([]byte, string, error) {
	storageName := fmt.Sprintf("lease_%d.html", leaseID)

	// 1. Try Storage
	if s.storage.Exists(storageName) {
		content, err := s.storage.Get(storageName)
		if err == nil {
			return content, "contract.html", nil
		}
		s.logger.Warn("failed to read stored lease", zap.Error(err))
	}

	// 2. Fallback: Generate
	return s.GenerateLeaseDocument(ctx, leaseID)
}

func (s *LeaseService) GenerateLeaseDocument(ctx context.Context, leaseID int32) ([]byte, string, error) {
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

		// Get Tenant
		tenant, err = q.GetUserById(ctx, l.TenantID.Int32)
		if err != nil {
			return fmt.Errorf("tenant not found: %w", err)
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
	charges, _ := prop.RentChargesAmount.Float64Value()

	total := rent.Float64 + charges.Float64

	data := LeaseTemplateData{
		BailleurNom:     fmt.Sprintf("%s %s", owner.LastName.String, owner.FirstName.String),
		BailleurEmail:   owner.Email,
		BailleurAdresse: "Non renseignée (voir profil)",

		LocataireNom:   fmt.Sprintf("%s %s", tenant.LastName.String, tenant.FirstName.String),
		LocataireEmail: tenant.Email,

		AdresseLogement: prop.Address,

		Surface:     "__",
		NbPieces:    "__",
		Dependances: "Aucune",
		ClasseDPE:   "__",

		DateDebut:        lease.StartDate.Time.Format("02/01/2006"),
		DureeBail:        "1 an (renouvelable)",
		LoyerHC:          fmt.Sprintf("%.2f", rent.Float64),
		Charges:          fmt.Sprintf("%.2f", charges.Float64),
		IsForfaitCharges: prop.IsFurnished.Bool, // Often fixed for furnished
		TotalMensuel:     fmt.Sprintf("%.2f", total),
		DepotGarantie:    fmt.Sprintf("%.2f", deposit.Float64),

		VilleSignature: "_______________",
		DateSignature:  time.Now().Format("02/01/2006"),
	}

	// 4. Execute Template
	tmpl, err := template.New("lease").Parse(string(content))
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, "", fmt.Errorf("failed to execute template: %w", err)
	}

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>Contrat de Location</title>
<style>
body { font-family: sans-serif; max-width: 800px; margin: 40px auto; padding: 20px; line-height: 1.6; }
h1, h2, h3 { color: #333; }
.page-break { page-break-before: always; }
strong { font-weight: bold; }
ul { margin-bottom: 1em; }
p { margin-bottom: 0.8em; }
</style>
</head>
<body>
<pre style="white-space: pre-wrap; font-family: serif; font-size: 11pt;">
%s
</pre>
</body>
</html>`, buf.String())

	return []byte(htmlContent), "contract.html", nil
}
