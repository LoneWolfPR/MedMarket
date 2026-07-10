-- Modify "pharmacies" table
ALTER TABLE "pharmacies" ADD COLUMN "code" character varying NOT NULL;
-- Create index "pharmacies_code_key" to table: "pharmacies"
CREATE UNIQUE INDEX "pharmacies_code_key" ON "pharmacies" ("code");
