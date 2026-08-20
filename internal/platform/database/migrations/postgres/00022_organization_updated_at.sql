-- +goose Up
ALTER TABLE organizations
  ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE organizations
  DROP COLUMN IF EXISTS updated_at;
