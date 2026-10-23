CREATE TABLE export_receipts (
 user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 exported_at timestamptz NOT NULL DEFAULT now()
);
