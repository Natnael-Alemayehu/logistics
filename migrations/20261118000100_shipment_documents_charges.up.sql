-- +goose Up
-- A shipment's documents and charges, the two tabs beside its history.

-- Files the office keeps with a load: a copy of the signed waybill, customs
-- papers, the customer's invoice. The file is in storage under
-- storage_key, served only through the dashboard. They hold customers'
-- details, so the keeping period removes them with the rest.
CREATE TABLE IF NOT EXISTS shipment_documents (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    kind text NOT NULL,
    name text NOT NULL,
    storage_key text NOT NULL,
    content_type text NOT NULL,
    size_bytes integer NOT NULL,
    uploaded_by uuid,
    uploaded_by_name text DEFAULT '' NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT shipment_documents_pkey PRIMARY KEY (id),
    CONSTRAINT shipment_documents_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT shipment_documents_shipment_id_fkey FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE,
    CONSTRAINT shipment_documents_uploaded_by_fkey FOREIGN KEY (uploaded_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT shipment_documents_kind_check CHECK (kind IN ('waybill', 'invoice', 'customs', 'delivery_note', 'photo', 'other')),
    CONSTRAINT shipment_documents_name_check CHECK (char_length(name) BETWEEN 1 AND 120),
    CONSTRAINT shipment_documents_size_check CHECK (size_bytes > 0)
);
CREATE INDEX IF NOT EXISTS idx_shipment_documents_shipment ON shipment_documents (tenant_id, shipment_id, created_at);

-- What the customer is charged for a load, line by line: freight, loading,
-- waiting time; a negative line is a discount. In birr.
CREATE TABLE IF NOT EXISTS shipment_charges (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    shipment_id uuid NOT NULL,
    description text NOT NULL,
    amount numeric(15,2) NOT NULL,
    created_by uuid,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT shipment_charges_pkey PRIMARY KEY (id),
    CONSTRAINT shipment_charges_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT shipment_charges_shipment_id_fkey FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE,
    CONSTRAINT shipment_charges_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT shipment_charges_description_check CHECK (char_length(description) BETWEEN 1 AND 120),
    CONSTRAINT shipment_charges_amount_check CHECK (amount <> 0 AND abs(amount) < 1000000000000)
);
CREATE INDEX IF NOT EXISTS idx_shipment_charges_shipment ON shipment_charges (tenant_id, shipment_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS shipment_charges;
DROP TABLE IF EXISTS shipment_documents;
