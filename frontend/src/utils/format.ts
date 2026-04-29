// Port des fonctions de Common.Formatting (Haxe) utilisées par le shop.

import type { Unit } from "../types/shop";

const DAYS = [
  "Dimanche",
  "Lundi",
  "Mardi",
  "Mercredi",
  "Jeudi",
  "Vendredi",
  "Samedi",
];

const MONTHS = [
  "Janvier",
  "Février",
  "Mars",
  "Avril",
  "Mai",
  "Juin",
  "Juillet",
  "Août",
  "Septembre",
  "Octobre",
  "Novembre",
  "Décembre",
];

/** Date longue : "Vendredi 1 Mai" (sans heure ni année). */
export function hDate(date: Date | string | null | undefined): string {
  if (!date) return "";
  const d = typeof date === "string" ? parseDateTime(date) : date;
  if (!d) return "";
  return `${DAYS[d.getDay()]} ${d.getDate()} ${MONTHS[d.getMonth()]}`;
}

/** "HH:MM" en local time. */
export function hHour(date: Date | string | null | undefined): string {
  if (!date) return "";
  const d = typeof date === "string" ? parseDateTime(date) : date;
  if (!d) return "";
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return `${hh}:${mm}`;
}

/** Format Haxe Formatting.formatNum : arrondi à 2 décimales, virgule à la française.
 *   - 1     → "1"
 *   - 1.5   → "1,50"
 *   - 1.555 → "1,56"
 */
export function formatNum(n: number | null | undefined): string {
  if (n == null) return "";
  const rounded = Math.round(n * 100) / 100;
  let out = String(rounded);
  if (out.indexOf(".") !== -1 && out.split(".")[1].length === 1) out += "0";
  return out.replace(".", ",");
}

/** Suffixe d'unité (port Haxe Formatting.unit). */
export function unitLabel(unit: Unit | number | null | undefined, quantity: number = 1): string {
  const u = typeof unit === "number" ? unitFromIndex(unit) : unit;
  switch (u) {
    case "Kilogram": return "Kg.";
    case "Gram": return "g.";
    case "Litre": return "L.";
    case "Centilitre": return "cl.";
    case "Millilitre": return "ml.";
    case "Piece":
    case null:
    case undefined:
      return quantity === 1 ? "pièce" : "pièces";
  }
}

/** "12,50 €" — format français avec virgule, 2 décimales. */
export function formatPrice(amount: number): string {
  return formatNum(amount) + " €";
}

/** Quantité + unité (port Haxe Formatting.smartQt utilisé sur la fiche produit).
 *   - 1 Piece → "1 pièce"
 *   - 0.5 Kg  → "0,50 Kg."
 */
export function smartQty(qty: number | null | undefined, unit: Unit | number | null | undefined): string {
  if (qty == null || qty === 0) return "";
  return `${formatNum(qty)} ${unitLabel(unit, qty)}`;
}

/** Prix par unité (port Haxe Formatting.pricePerUnit).
 * Pour les très petits prix au gramme / cl / ml, convertit en /Kg ou /L.
 */
export function pricePerUnit(price: number, qty: number | null | undefined, unit: Unit | number | null | undefined): string {
  if (!qty || !price) return "";
  let u = typeof unit === "number" ? unitFromIndex(unit) : unit ?? null;
  let p = price / qty;
  if (p < 1) {
    if (u === "Gram") { p *= 1000; u = "Kilogram"; }
    else if (u === "Centilitre") { p *= 100; u = "Litre"; }
    else if (u === "Millilitre") { p *= 1000; u = "Litre"; }
  }
  return `${formatNum(p)} €/${unitLabel(u, qty)}`;
}

// ─── helpers ────────────────────────────────────────────────────────────────

const UNIT_INDEX: Unit[] = [
  "Piece",
  "Kilogram",
  "Gram",
  "Litre",
  "Centilitre",
  "Millilitre",
];

function unitFromIndex(idx: number): Unit | null {
  return UNIT_INDEX[idx] ?? null;
}

/** Parse "YYYY-MM-DD HH:MM:SS" (format renvoyé par /api/shop/init). */
export function parseDateTime(s: string): Date | null {
  // Format attendu : "YYYY-MM-DD HH:MM:SS" (length 19) ou "YYYY-MM-DDTHH:MM:SS+TZ"
  if (!s) return null;
  if (s.length === 19 && s[10] === " ") {
    const [date, time] = s.split(" ");
    const [y, m, d] = date.split("-").map(Number);
    const [hh, mm, ss] = time.split(":").map(Number);
    return new Date(y, m - 1, d, hh, mm, ss);
  }
  const d = new Date(s);
  return isNaN(d.getTime()) ? null : d;
}
