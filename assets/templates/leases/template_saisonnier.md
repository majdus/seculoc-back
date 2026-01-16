# CONTRAT DE LOCATION SAISONNIÈRE

(Meublé de Tourisme - Code Civil)

### I. PARTIES

**BAILLEUR :** {{.BailleurNom}}
Contact : {{.BailleurEmail}}

**LOCATAIRE :** {{.LocataireNom}}
Contact : {{.LocataireEmail}}

---

### II. OBJET ET DURÉE

**Adresse :** {{.AdresseLogement}}
**Type :** {{.TypeHabitat}} / Capacité max : {{.CapaciteMax}} personnes.
**N° Enregistrement Mairie :** {{.NumEnregistrement}}

**SÉJOUR FERME :**

- **Arrivée le :** {{.DateDebut}} à partir de {{.HeureArrivee}}
- **Départ le :** {{.DateFin}} au plus tard à {{.HeureDepart}}

---

### III. PRIX ET PAIEMENT

**1. Prix du séjour :** {{.PrixTotal}} € (Charges comprises).
**2. Taxe de séjour :** {{.TaxeSejour}} € (à régler en sus).
**3. Total à payer :** {{.TotalGeneral}} €.

**4. Modalités :**

- **Acompte (Réservation) :** {{.Acompte}} € payé ce jour.
- **Solde restant :** {{.Solde}} € à payer avant l'entrée dans les lieux.

---

### IV. DÉPÔT DE GARANTIE

Une caution de **{{.DepotGarantie}} €** sera remise à l'arrivée.
Elle sera restituée au départ ou sous 15 jours maximum, déduction faite des éventuels dégâts.

---

### V. ANNULATION

- Par le Locataire > 30 jours : Acompte restitué.
- Par le Locataire < 30 jours : Acompte conservé par le Bailleur.
- Non-présentation : Totalité du séjour due.

---

### VI. RÈGLEMENT

Logement **Non Fumeur**. Animaux : {{if .AnimauxAutorises}}Autorisés{{else}}Interdits{{end}}.
Interdiction de dépasser la capacité de {{.CapaciteMax}} personnes.

<br>

**Fait à {{.VilleSignature}}, le {{.DateSignature}}**

<br>
<br>

**LE BAILLEUR** ....................................... **LE LOCATAIRE**
