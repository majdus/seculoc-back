# Séculoc Backend

**Séculoc** est une plateforme de gestion locative et de tiers de confiance, conçue pour sécuriser les revenus des propriétaires et protéger les locataires. Ce repository contient le backend de l'application, développé en **Go**.

## 🚀 Fonctionnalités Clés (Backend)

- **Architecture Hexagonale (Ports & Adapters)** : Separation claire entre `Core` (Métier), `Adapter` (Infra) et `Platform` (Libs).
- **Performance & Zero-Allocation** : Utilisation de `uber-go/zap` pour le logging structuré haute performance.
- **Persistence Type-Safe** : Utilisation de `sqlc` avec `pgx/v5` pour générer du code Go type-safe à partir de requêtes SQL.
- **Base de Données** : PostgreSQL 15.
- **Configuration** : Gestion centralisée via `Viper` (.env).

## 🛠 Prérequis

- **Go** : Version 1.21+
- **Docker & Docker Compose** : Pour la base de données PostgreSQL locale.
- **Make** : Pour l'exécution des commandes d'automatisation.
- **sqlc** (Optionnel pour le dev) : Pour régénérer le code SQL (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`).

## ⚙️ Installation & Configuration

1.  **Cloner le projet**

    ```bash
    git clone git@github.com-majdus:majdus/seculoc-back.git
    cd seculoc-back
    ```

2.  **Configuration**
    Copiez le fichier d'exemple et ajustez si nécessaire :

    ```bash
    cp .env.example .env
    ```

3.  **Démarrer l'infrastructure (Base de données)**
    ```bash
    make docker-up
    ```
    Cela va lancer un conteneur PostgreSQL et **initialiser automatiquement la structure de la base de données** (via `db/schemas.sql`).

## 🔑 Variables d'Environnement

Le fichier `.env` configure l'application. Voici les clés principales :

| Variable         | Description                                 | Défaut              |
| :--------------- | :------------------------------------------ | :------------------ |
| `SERVER_ADDRESS` | Port d'écoute du serveur                    | `:8080`             |
| `DB_HOST`        | Hôte PostgreSQL                             | `localhost`         |
| `DB_USER`        | Utilisateur BDD                             | `postgres`          |
| `DB_PASSWORD`    | Mot de passe BDD                            | `password`          |
| `DB_NAME`        | Nom de la BDD                               | `seculoc`           |
| `JWT_SECRET`     | Clé secrète pour signer les tokens JWT      | `change_me_in_prod` |
| `ENV`            | Environnement (`development`, `production`) | `development`       |

## 📡 API Endpoints

L'API expose les ressources suivantes sur `/api/v1`.

### Authentification

- `POST /api/v1/auth/register` : Inscription d'un nouvel utilisateur.
- `POST /api/v1/auth/login` : Connexion (Retourne un JWT).
- `POST /api/v1/auth/switch-context` : Changer de contexte (Owner <-> Tenant).

### Invitations (Protégé par JWT)

- `POST /api/v1/invitations` : Inviter un locataire.
- `POST /api/v1/invitations/accept` : Accepter une invitation.

### Properties (Protégé par JWT)

- `POST /api/v1/properties` : Créer un bien (vérifie les quotas).
- `GET /api/v1/properties` : Lister ses biens.

### Subscriptions (Protégé par JWT)

- `POST /api/v1/subscriptions` : Souscrire à un plan (Discovery, Serenity, Premium).
- `POST /api/v1/subscriptions/upgrade` : Acheter des slots supplémentaires.

### Solvency (Protégé par JWT)

- `POST /api/v1/solvency/check` : Lancer une vérification de solvabilité (Coût : 1 crédit).
- `POST /api/v1/solvency/credits` : Acheter des crédits (ex: "pack_20").

## ▶️ Démarrage

Pour lancer le serveur backend :

```bash
make run
```

Le serveur démarrera sur `http://localhost:8080`.
Vous pouvez vérifier la santé du service via : `http://localhost:8080/health`.

## 🧪 Tests

Lancer la suite de tests unitaires :

```bash
make test
```

```bash
make test
```

## 🧹 Qualité de Code

Pour maintenir la base de code propre et standardisée :

- **Formatage** :
  ```bash
  go fmt ./...
  ```
- **Analyse Statique (Linting)** :
  ```bash
  go vet ./...
  ```

## 🏗 Commandes Utiles (Makefile)

- `make build` : Compile le binaire dans `bin/server`.
- `make run` : Lance l'application.
- `make test` : Lance tous les tests.
- `make docker-up` : Démarre la base de données (Docker).
- `make docker-down` : Arrête la base de données.
- `make db-reset` : ⚠️ **Danger** : Supprime et recrée toutes les tables (Perte de données).
- `make sqlc` : Régénère le code Go à partir des fichiers SQL (`db/query.sql` et `db/schemas.sql`).

## 📂 Structure du Projet

```
.
├── cmd/server/            # Point d'entrée de l'application (main.go)
├── db/                    # Schémas SQL et requêtes (schemas.sql, query.sql)
├── internal/
│   ├── adapter/           # Adaptateurs (HTTP, Storage/Postgres)
│   ├── core/              # Cœur métier (Service, Domain, Ports)
│   └── platform/          # Code technique transverse (Logger, Utils)
├── Makefile               # Automatisation
├── sqlc.yaml              # Configuration SQLC
└── docker-compose.yml     # Stack locale
```

## 📝 Licence

Propriété de Séculoc. Tous droits réservés.
