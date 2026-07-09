-- Create "pharmacies" table
CREATE TABLE "pharmacies" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "name" character varying NOT NULL,
  "contact_phone" character varying NOT NULL,
  "npi" character varying NOT NULL,
  "dea" character varying NOT NULL,
  "ncpdp" character varying NOT NULL,
  "address_street1" character varying NOT NULL,
  "address_street2" character varying NULL,
  "address_city" character varying NOT NULL,
  "address_state" character varying NOT NULL,
  "address_zip" character varying NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create index "pharmacies_dea_key" to table: "pharmacies"
CREATE UNIQUE INDEX "pharmacies_dea_key" ON "pharmacies" ("dea");
-- Create index "pharmacies_ncpdp_key" to table: "pharmacies"
CREATE UNIQUE INDEX "pharmacies_ncpdp_key" ON "pharmacies" ("ncpdp");
-- Create index "pharmacies_npi_key" to table: "pharmacies"
CREATE UNIQUE INDEX "pharmacies_npi_key" ON "pharmacies" ("npi");
