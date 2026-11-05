// Money on screen (design doc "Time and locale"): amounts are integer minor units, and the
// currency's own exponent says how many decimals the unit has.

/** The number of decimals of a currency's minor unit (2 for dollars, 0 for yen). */
export function exponentOf(currency: string, locale: string): number {
	return (
		new Intl.NumberFormat(locale, { style: 'currency', currency }).resolvedOptions()
			.maximumFractionDigits ?? 2
	);
}

/** `minor` units written as money in the business's currency and locale. */
export function formatMoney(minor: number, currency: string, locale: string): string {
	const exponent = exponentOf(currency, locale);
	return new Intl.NumberFormat(locale, { style: 'currency', currency }).format(
		minor / 10 ** exponent
	);
}

/** What she typed as minor units, or null when it is not a plain non-negative amount with at most
 *  the currency's decimals. A comma or a point is the decimal mark. */
export function parseAmount(text: string, exponent: number): number | null {
	const match = /^(\d+)(?:[.,](\d+))?$/.exec(text.trim());
	const whole = match?.[1];
	if (whole === undefined) {
		return null;
	}
	const fraction = match?.[2] ?? '';
	if (fraction.length > exponent) {
		return null;
	}
	return Number(whole + fraction.padEnd(exponent, '0'));
}

/** `minor` units as the text of an amount field. */
export function amountText(minor: number, exponent: number): string {
	return (minor / 10 ** exponent).toFixed(exponent);
}
