import express from "express";
import { readFile } from "node:fs/promises";
import { randomUUID } from "node:crypto";

const PORT = process.env.PORT || 8080;
const API_KEY = process.env.API_KEY || "pharmacy-a-dev-key";
const ERROR_RATE = 0.05; // ~5% of requests fail, to exercise adapter error handling.

// Load the fixed catalog once at boot. Prices are stored as ranges; each
// search randomizes within [minCents, maxCents] to simulate live quoting.
const catalog = JSON.parse(
  await readFile(new URL("./catalog.json", import.meta.url), "utf8"),
);

const randInt = (min, max) => Math.floor(Math.random() * (max - min + 1)) + min;
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const app = express();
app.use(express.json());

// Pharmacy A's auth scheme: a shared secret on the X-Api-Key header.
app.use("/api", (req, res, next) => {
  if (req.get("X-Api-Key") !== API_KEY) {
    return res.status(401).json({ error: "unauthorized" });
  }
  next();
});

// Search returns quotes with money as integer cents.
app.post("/api/v1/search", async (req, res) => {
  await delay(randInt(200, 500)); // simulated upstream latency
  if (Math.random() < ERROR_RATE) {
    return res.status(500).json({ error: "internal_error" });
  }
  const { drug = "", strength = "" } = req.body ?? {};
  const results = catalog
    .filter(
      (m) =>
        m.drug.toLowerCase() === drug.toLowerCase() &&
        (strength === "" || m.strength.toLowerCase() === strength.toLowerCase()),
    )
    .map((m) => ({
      sku: m.sku,
      drug: m.drug,
      strength: m.strength,
      priceCents: randInt(m.minCents, m.maxCents),
      currency: "USD",
      inStock: true,
      shipsInDays: m.shipsInDays,
    }));
  res.json({ results });
});

// Pharmacy A takes the destination as a flat `shipping` object with its own
// field names — pharmacy B nests the same data under `recipient` with
// different keys. street2 is the only optional field.
const requiredShippingFields = ["name", "street1", "city", "state", "zip"];

const missingShippingFields = (shipping) =>
  requiredShippingFields.filter(
    (f) => typeof shipping?.[f] !== "string" || shipping[f].trim() === "",
  );

// Pharmacy A honors an Idempotency-Key header: a retried order with the same
// key replays the original response instead of placing a second order. This is
// what lets the backend safely retry an ambiguous outcome (e.g. a lost
// response). Pharmacy B deliberately has no equivalent. In-memory is fine for a
// throwaway mock.
const idempotencyStore = new Map();

// Order confirms against a SKU and returns a tracking id the backend can
// later register with the shipping service.
app.post("/api/v1/order", async (req, res) => {
  await delay(randInt(200, 500));

  // A repeated key replays the stored order deterministically — and skips the
  // random-failure roll below — so a retry never places a duplicate or fails.
  const idempotencyKey = req.get("Idempotency-Key");
  if (idempotencyKey && idempotencyStore.has(idempotencyKey)) {
    return res.status(201).json(idempotencyStore.get(idempotencyKey));
  }

  if (Math.random() < ERROR_RATE) {
    return res.status(500).json({ error: "internal_error" });
  }
  const { sku, quantity = 1, shipping } = req.body ?? {};
  const missing = missingShippingFields(shipping);
  if (missing.length > 0) {
    return res.status(400).json({ error: "invalid_shipping", missing });
  }
  const item = catalog.find((m) => m.sku === sku);
  if (!item) {
    return res.status(404).json({ error: "unknown_sku" });
  }
  const unitCents = randInt(item.minCents, item.maxCents);
  const receipt = {
    orderId: `ord_${randomUUID().slice(0, 8)}`,
    trackingId: `RXD${randomUUID().replace(/-/g, "").slice(0, 12).toUpperCase()}`,
    status: "CONFIRMED",
    totalCents: unitCents * quantity,
  };
  // Only a successful placement is cached; a 400/404/500 leaves no entry, so a
  // corrected or retried request is processed fresh.
  if (idempotencyKey) {
    idempotencyStore.set(idempotencyKey, receipt);
  }
  res.status(201).json(receipt);
});

app.get("/healthz", (_req, res) => res.json({ status: "ok" }));

app.listen(PORT, () => console.log(`mock-pharmacy-a listening on :${PORT}`));
