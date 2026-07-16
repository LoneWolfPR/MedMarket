-- Create "orders" table
CREATE TABLE "orders" (
  "id" uuid NOT NULL DEFAULT gen_random_uuid(),
  "status" character varying NOT NULL,
  "pharmacy_order_id" character varying NULL,
  "tracking_id" character varying NULL,
  "quantity" bigint NOT NULL,
  "price_paid_cents" bigint NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "offer_id" uuid NOT NULL,
  "prescription_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "orders_offers_orders" FOREIGN KEY ("offer_id") REFERENCES "offers" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "orders_prescriptions_orders" FOREIGN KEY ("prescription_id") REFERENCES "prescriptions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "order_status_valid" CHECK ((status)::text = ANY ((ARRAY['placed'::character varying, 'confirmed'::character varying, 'shipped'::character varying, 'delivered'::character varying, 'failed'::character varying, 'canceled'::character varying])::text[]))
);
-- Create index "order_offer_id" to table: "orders"
CREATE INDEX "order_offer_id" ON "orders" ("offer_id");
-- Create index "order_prescription_id" to table: "orders"
CREATE INDEX "order_prescription_id" ON "orders" ("prescription_id");
