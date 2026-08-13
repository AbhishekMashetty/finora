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

export function formatDate(
  dateStr: string,
  options: Intl.DateTimeFormatOptions = { year: "numeric", month: "short", day: "numeric" },
  locale = DEFAULT_LOCALE,
): string {
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return dateStr;
  return new Intl.DateTimeFormat(locale, options).format(date);
}
