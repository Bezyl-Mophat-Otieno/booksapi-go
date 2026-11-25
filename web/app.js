"use strict";

// Pesapal sends the customer back to this page (the callback URL) with these
// query parameters once the payment flow ends.
const callbackParams = new URLSearchParams(window.location.search);
const callback = callbackParams.get("OrderTrackingId")
  ? {
    trackingId: callbackParams.get("OrderTrackingId"),
    merchantReference: callbackParams.get("OrderMerchantReference") || "",
  }
  : null;

const STATUS_POLL_ATTEMPTS = 5;
const STATUS_POLL_DELAY_MS = 4000;

const money = new Intl.NumberFormat("en-KE", { style: "currency", currency: "KES" });

const el = {
  refresh: document.getElementById("refresh-orders"),
  result: document.getElementById("payment-result"),
  panel: document.getElementById("payment-panel"),
  title: document.getElementById("payment-title"),
  frame: document.getElementById("payment-frame"),
  close: document.getElementById("close-payment"),
  message: document.getElementById("orders-message"),
  table: document.getElementById("orders-table"),
  body: document.getElementById("orders-body"),
};

// The payment currently open in the iframe, if any.
let activePayment = null;
let pollTimer = null;

if (callback && window.self !== window.top) {
  // Pesapal redirected inside the iframe: hand the result to the page around it.
  document.body.textContent = "Payment finished. Returning to orders…";
  try {
    window.parent.handlePaymentCallback(callback);
  } catch (err) {
    window.top.location.href = window.location.href;
  }
} else {
  init();
}

function init() {
  el.refresh.addEventListener("click", loadOrders);
  el.close.addEventListener("click", () => {
    const payment = activePayment;
    closePaymentPanel();
    // The customer may have paid without being redirected back; check anyway.
    if (payment) checkPaymentStatus(payment);
  });

  loadOrders();

  if (callback) {
    // Drop the query string so a page reload does not re-run the check.
    window.history.replaceState(null, "", window.location.pathname);
    checkPaymentStatus(callback);
  }
}

// Called from the iframe when Pesapal redirects inside it.
window.handlePaymentCallback = (result) => {
  setTimeout(() => {
    closePaymentPanel();
    checkPaymentStatus(result);
  }, 0);
};

async function api(path, options) {
  const response = await fetch(path, {
    headers: { Accept: "application/json" },
    ...options,
  });
  const data = await response.json().catch(() => null);
  if (!response.ok) {
    throw new Error((data && data.error) || `Request failed (${response.status})`);
  }
  return data;
}

async function loadOrders() {
  el.refresh.disabled = true;
  try {
    const orders = await api("/orders");
    renderOrders(orders || []);
  } catch (err) {
    el.table.hidden = true;
    el.message.hidden = false;
    el.message.textContent = `Could not load orders: ${err.message}`;
  } finally {
    el.refresh.disabled = false;
  }
}

function renderOrders(orders) {
  el.body.replaceChildren();
  if (orders.length === 0) {
    el.table.hidden = true;
    el.message.hidden = false;
    el.message.textContent = "No orders yet.";
    return;
  }

  for (const order of orders) {
    const customer = document.createElement("td");
    customer.append(`${order.customer_firstname} ${order.customer_lastname}`.trim());
    const email = document.createElement("span");
    email.className = "customer-email";
    email.textContent = order.customer_email;
    customer.append(email);

    const action = document.createElement("td");
    action.className = "num";
    if (order.status === "placed") {
      const pay = document.createElement("button");
      pay.type = "button";
      pay.className = "button button-primary";
      pay.textContent = "Pay";
      pay.addEventListener("click", () => startPayment(order, pay));
      action.append(pay);
    }

    const row = document.createElement("tr");
    row.append(
      cell(`#${order.id}`),
      customer,
      cell(money.format(order.total_cents / 100), "num"),
      badgeCell(order.status),
      cell(new Date(order.placed_at).toLocaleString()),
      action,
    );
    el.body.append(row);
  }

  el.message.hidden = true;
  el.table.hidden = false;
}

function cell(text, className) {
  const td = document.createElement("td");
  td.textContent = text;
  if (className) td.className = className;
  return td;
}

function badgeCell(status) {
  const td = document.createElement("td");
  td.append(badge(status));
  return td;
}

function badge(status) {
  const span = document.createElement("span");
  const known = ["placed", "paid", "dispatched", "cancelled", "pending", "completed", "failed", "invalid", "reversed"];
  span.className = `badge badge-${known.includes(status) ? status : "unknown"}`;
  span.textContent = status || "unknown";
  return span;
}

async function startPayment(order, button) {
  button.disabled = true;
  button.textContent = "Starting…";
  try {
    const payment = await api(`/orders/${order.id}/payment-request`, { method: "POST" });
    activePayment = {
      orderId: order.id,
      trackingId: payment.order_tracking_id,
      merchantReference: payment.merchant_reference,
    };
    hideResult();
    el.title.textContent = `Paying order #${order.id} · ${money.format(order.total_cents / 100)}`;
    el.frame.src = payment.redirect_url;
    el.panel.hidden = false;
    el.panel.scrollIntoView({ behavior: "smooth", block: "start" });
  } catch (err) {
    showResult("error", "Could not start the payment", err.message);
  } finally {
    button.disabled = false;
    button.textContent = "Pay";
  }
}

function closePaymentPanel() {
  activePayment = null;
  el.panel.hidden = true;
  el.frame.src = "about:blank";
}

async function checkPaymentStatus(payment, attempt = 1) {
  clearTimeout(pollTimer);
  if (attempt === 1) showResult("info", "Checking payment status…");

  const query = new URLSearchParams({ OrderTrackingId: payment.trackingId });
  if (payment.merchantReference) query.set("OrderMerchantReference", payment.merchantReference);

  let data;
  try {
    data = await api(`/pesapal/transaction-status?${query}`);
  } catch (err) {
    showResult("error", "Could not check the payment status", err.message, () => checkPaymentStatus(payment));
    return;
  }

  const transaction = data.transaction;
  const status = transaction ? transaction.status : (data.payment_status_description || "").toLowerCase();
  const orderLabel = transaction ? `order #${transaction.order_id}` : `reference ${payment.merchantReference || payment.trackingId}`;

  // Right after the redirect Pesapal may not have settled the payment yet, so
  // keep asking for a little while before reporting it as not completed.
  const settled = status === "completed" || status === "failed" || status === "reversed";
  if (!settled && attempt < STATUS_POLL_ATTEMPTS) {
    showResult("pending", "Waiting for Pesapal to confirm the payment…", `Check ${attempt} of ${STATUS_POLL_ATTEMPTS}.`);
    pollTimer = setTimeout(() => checkPaymentStatus(payment, attempt + 1), STATUS_POLL_DELAY_MS);
    return;
  }

  const details = [
    data.payment_method && `Method: ${data.payment_method}`,
    data.confirmation_code && `Confirmation: ${data.confirmation_code}`,
    data.description,
  ].filter(Boolean).join(" · ");

  switch (status) {
    case "completed":
      showResult("success", `Payment received for ${orderLabel}.`, details);
      break;
    case "failed":
      showResult("error", `Payment failed for ${orderLabel}.`, details, () => checkPaymentStatus(payment));
      break;
    case "reversed":
      showResult("error", `Payment for ${orderLabel} was reversed.`, details);
      break;
    default:
      showResult(
        "pending",
        `Payment for ${orderLabel} has not been completed yet.`,
        "If you finished paying, check again in a moment.",
        () => checkPaymentStatus(payment),
      );
  }
  loadOrders();
}

function showResult(kind, title, detail, retry) {
  const heading = document.createElement("strong");
  heading.textContent = title;
  el.result.replaceChildren(heading);
  if (detail) {
    const p = document.createElement("p");
    p.textContent = detail;
    el.result.append(p);
  }
  if (retry) {
    const p = document.createElement("p");
    const button = document.createElement("button");
    button.type = "button";
    button.className = "button button-secondary";
    button.textContent = "Check again";
    button.addEventListener("click", retry);
    p.append(button);
    el.result.append(p);
  }
  el.result.className = `notice notice-${kind}`;
  el.result.hidden = false;
}

function hideResult() {
  clearTimeout(pollTimer);
  el.result.hidden = true;
}
