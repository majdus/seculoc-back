# CONTRAT DE LOCATION MEUBLÉE

(Titre Ier bis de la loi du 6 juillet 1989)

### I. DÉSIGNATION DES PARTIES

**LE BAILLEUR :** {{.BailleurNom}}
Adresse : {{.BailleurAdresse}} / Email : {{.BailleurEmail}}

**LE LOCATAIRE :** {{.LocataireNom}}
Email : {{.LocataireEmail}}

---

### II. OBJET DU CONTRAT

**1. Le Logement :**
Adresse : {{.AdresseLogement}}
Surface : {{.Surface}} m² / Pièces : {{.NbPieces}}
Dépendances : {{.Dependances}}

**2. Performance Énergétique :**
Classement DPE : **Lettre {{.ClasseDPE}}**.

**3. Mobilier :**
Le logement est loué meublé (voir Inventaire détaillé en annexe).

---

### III. DURÉE

**1. Prise d'effet :** {{.DateDebut}}
**2. Durée :** {{.DureeBail}} (ex: 1 AN ou 9 MOIS étudiant).
**3. Reconduction :** Tacite par périodes d'un an (sauf bail étudiant 9 mois non renouvelable).

---

### IV. LOYER ET CHARGES

**1. Loyer mensuel :** {{.LoyerHC}} € HC.
**2. Charges :** {{.Charges}} € (Type : {{if .IsForfaitCharges}}Forfait{{else}}Provision{{end}}).
**3. Total mensuel :** **{{.TotalMensuel}} €.**

**4. Révision :** Annuelle selon IRL. Interdite si classe DPE F ou G (Actuel : {{.ClasseDPE}}).

---

### V. DÉPÔT DE GARANTIE

Montant : **{{.DepotGarantie}} €.**
_(Soit maximum 2 mois de loyer hors charges)._

---

### VI. CONGÉ

- **Locataire :** Préavis d'**1 MOIS** à tout moment.
- **Bailleur :** Préavis de **3 MOIS** à l'échéance (Vente, Reprise, Motif sérieux).

---

### VII. ANNEXES OBLIGATOIRES

1. État des lieux et **INVENTAIRE DU MOBILIER**.
2. Dossier Technique (DPE {{.ClasseDPE}}, ERP...).

<br>

**Fait à {{.VilleSignature}}, le {{.DateSignature}}**

<br>
<br>

**LE BAILLEUR** ....................................... **LE LOCATAIRE**
