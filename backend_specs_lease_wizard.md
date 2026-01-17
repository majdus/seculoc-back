# Backend Requirements for Lease Wizard Workflow

To support the new "Lease Creation Wizard" on the frontend, the backend needs to evolve from a simple invitation system to a stateful lease creation process.

## 1. New Endpoint: Create Draft Lease
We need an endpoint that can accept all the details of a lease *before* the tenant exists in the system.

- **Endpoint**: `POST /api/v1/leases/draft`
- **Auth**: Owner only.
- **Payload**:
  ```json
  {
    "property_id": 123,
    "tenant_info": {
      "first_name": "Jean",
      "last_name": "Dupont",
      "email": "jean@example.com",
      "phone": "+33612345678" // Optional
    },
    "terms": {
      "start_date": "2026-03-01",
      "end_date": "2027-03-01", // Nullable if indefinite
      "rent_amount": 950.00,
      "charges_amount": 50.00,
      "deposit_amount": 950.00,
      "payment_day": 5
    },
    "clauses": [
      "Animaux autorisés",
      "Non fumeur"
    ]
  }
  ```

- **Behavior**:
  1.  Validate inputs.
  2.  Create a [Lease](file:///home/mchatti/perso/seculoc/seculoc-front/src/api/modules/leases.ts#3-24) record with status `DRAFT` (or `PENDING_SIGNATURE`).
  3.  Store `tenant_info` temporarily (either in a JSON column `draft_data` on the Lease table, or create a `PendingUser` record).
  4.  **Trigger Invitation**: Send the invitation email to `tenant_info.email`. The invitation link should include a token linked to this specific Lease (e.g., `?lease_token=xyz` or link the existing invitation token to this lease_id).

- **Response**:
  ```json
  {
    "lease_id": 456,
    "status": "draft",
    "message": "Lease created and invitation sent."
  }
  ```

## 2. Updated Endpoint: Get Lease Details
The `GET /api/v1/leases/:id` endpoint must be able to return the `tenant_info` from the draft data if the real `tenant_id` is still null.

- **Response Update**:
  If `tenant_id` is null, populate the `tenant` object in the response using the stored draft details, so the frontend can still display "Jean Dupont (Invité)".

## 3. Invitation Acceptance Flow
When the user clicks the invitation link and registers:
1.  The system identifies the pending Lease(s) associated with this email or token.
2.  Optimally, upon account creation, update the [Lease](file:///home/mchatti/perso/seculoc/seculoc-front/src/api/modules/leases.ts#3-24) record:
    *   Set `tenant_id` to the new User's ID.
    *   Update status from `DRAFT` to `ACTIVE` (or `PENDING_SIGNATURE` if you want a signature step).

## 4. PDF Generation
Ensure the PDF generation engine (`GET /leases/:id/download`) uses the 'draft' terms (rent, dates) stored in the lease, rather than just the generic property data.
