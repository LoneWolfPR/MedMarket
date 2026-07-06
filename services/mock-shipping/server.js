import express from "express";

const PORT = process.env.PORT || 8080;
// Compressed shipping timeline: four stages, one per interval (~15s → ~60s
// total). Lower the interval in test mode for faster, deterministic runs.
const INTERVAL_MS = Number(process.env.SHIPPING_INTERVAL_MS) || 15000;
const STAGES = ["picked_up", "in_transit", "out_for_delivery", "delivered"];

// In-memory shipment registry: trackingId → { status, callbackUrl, history }.
const shipments = new Map();

const app = express();
app.use(express.json());

// Register a shipment: the backend calls this after placing a pharmacy order,
// handing us the pharmacy's tracking id and a webhook URL to notify.
app.post("/api/register-webhook", (req, res) => {
  const { trackingId, callbackUrl } = req.body ?? {};
  if (!trackingId || !callbackUrl) {
    return res.status(400).json({ error: "trackingId and callbackUrl are required" });
  }
  shipments.set(trackingId, { status: "registered", callbackUrl, history: [] });
  scheduleProgression(trackingId);
  res.status(202).json({ registered: true, trackingId });
});

app.get("/api/status/:trackingId", (req, res) => {
  const shipment = shipments.get(req.params.trackingId);
  if (!shipment) {
    return res.status(404).json({ error: "unknown trackingId" });
  }
  res.json({
    trackingId: req.params.trackingId,
    status: shipment.status,
    history: shipment.history,
  });
});

app.get("/healthz", (_req, res) => res.json({ status: "ok" }));

// Walk a shipment through the stages, firing a webhook at each step.
function scheduleProgression(trackingId) {
  let stage = 0;
  const tick = async () => {
    const shipment = shipments.get(trackingId);
    if (!shipment || stage >= STAGES.length) return;
    const status = STAGES[stage++];
    const occurredAt = new Date().toISOString();
    shipment.status = status;
    shipment.history.push({ status, occurredAt });
    await notify(shipment.callbackUrl, { trackingId, status, occurredAt });
    if (stage < STAGES.length) setTimeout(tick, INTERVAL_MS);
  };
  setTimeout(tick, INTERVAL_MS);
}

async function notify(url, payload) {
  try {
    await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
  } catch (err) {
    // Best-effort webhook — a mock shouldn't crash if the receiver is down.
    console.error(`webhook to ${url} failed:`, err.message);
  }
}

app.listen(PORT, () => console.log(`mock-shipping listening on :${PORT}`));
