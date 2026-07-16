-- Modify "prescriptions" table
ALTER TABLE "prescriptions" ADD CONSTRAINT "prescriptions_users_prescriptions" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create "offers" table
CREATE TABLE "offers" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "price_cents" bigint NOT NULL,
  "pharmacy_item_id" character varying NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "pharmacy_id" uuid NOT NULL,
  "prescription_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "offers_pharmacies_offers" FOREIGN KEY ("pharmacy_id") REFERENCES "pharmacies" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "offers_prescriptions_offers" FOREIGN KEY ("prescription_id") REFERENCES "prescriptions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "offer_pharmacy_id" to table: "offers"
CREATE INDEX "offer_pharmacy_id" ON "offers" ("pharmacy_id");
-- Create index "offer_prescription_id" to table: "offers"
CREATE INDEX "offer_prescription_id" ON "offers" ("prescription_id");
