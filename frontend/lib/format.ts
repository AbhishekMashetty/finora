// Locale-aware number/date formatting via Intl — replaces the app-wide
// pattern of `amount.toFixed(2) + " " + currency` string concatenation,
// which doesn't group thousands, doesn't place the currency symbol per
// locale convention, and breaks for currencies with a different number of
// minor units (e.g. JPY has none).

const DEFAULT_LOCALE = "en-US";

export function formatCurrency(amount: number, currency: string, locale = DEFAULT_LOCALE): string {
  try {
    return new Intl.NumberFormat(locale, { style: "currency", currency }).format(amount);
  } catch {
    // Intl throws on a currency code it doesn't recognize — settings.currency
    // is validated server-side only as "3 uppercase letters", not a real
    // ISO-4217 lookup, so this fallback keeps an unusual code from crashing
    // the page instead of rendering a number.
    return `${amount.toFixed(2)} ${currency}`;
  }
}

/** Transaction amounts are always stored positive with a separate `type` for polarity. */
export function formatSignedAmount(
  amount: number,
  currency: string,
  type: "income" | "expense",
  locale = DEFAULT_LOCALE,
): string {
  const formatted = formatCurrency(Math.abs(amount), currency, locale);
  return type === "income" ? `+${formatted}` : `−${formatted}`;
}

/** Plain grouped number, no currency symbol — for amounts with no currency
 * field to format against (e.g. Budget, which budget-service stores without
 * one; guessing a currency here would misrepresent a non-USD user's data). */
export function formatNumber(
  amount: number,
  options: Intl.NumberFormatOptions = { minimumFractionDigits: 2, maximumFractionDigits: 2 },
  locale = DEFAULT_LOCALE,
): string {
  return new Intl.NumberFormat(locale, options).format(amount);
}

export function formatDate(
  dateStr: string,
  options: Intl.DateTimeFormatOptions = { year: "numeric", month: "short", day: "numeric" },
  locale = DEFAULT_LOCALE,
): string {
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return dateStr;
  return new Intl.DateTimeFormat(locale, options).format(date);
}
