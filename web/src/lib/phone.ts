// How a phone number is shown (release-1 bug L7). Display only: the stored E.164 value, the
// search digits and the wa.me link never change.

/** A country's calling code and how its national number is grouped for reading. */
type Plan = { code: string; groups: readonly number[] };

/** The calling codes the business is likely to meet (Costa Rica and its neighbours, the rest of
 *  Latin America, Spain and North America). E.164 codes are prefix-free, so the first match is the
 *  only one. A number of another country, or of an unexpected length, shows as stored. Mexico
 *  (+52) is left out: its area codes are 2 or 3 digits, so one fixed grouping would split some
 *  numbers wrongly. */
const PLANS: readonly Plan[] = [
	{ code: '1', groups: [3, 3, 4] },
	{ code: '34', groups: [3, 3, 3] },
	{ code: '51', groups: [3, 3, 3] },
	{ code: '56', groups: [1, 4, 4] },
	{ code: '57', groups: [3, 3, 4] },
	{ code: '58', groups: [3, 3, 4] },
	{ code: '502', groups: [4, 4] },
	{ code: '503', groups: [4, 4] },
	{ code: '504', groups: [4, 4] },
	{ code: '505', groups: [4, 4] },
	{ code: '506', groups: [4, 4] },
	{ code: '507', groups: [4, 4] },
	{ code: '593', groups: [2, 3, 4] },
	{ code: '595', groups: [3, 3, 3] },
	{ code: '598', groups: [2, 3, 3] }
];

const E164 = /^\+(\d+)$/;

/** `+50688887777` as `+506 8888 7777`: the calling code, then the national number in its
 *  country's groups. Anything else comes back unchanged. */
export function formatPhone(e164: string): string {
	const digits = E164.exec(e164)?.[1];
	if (digits === undefined) {
		return e164;
	}
	const plan = PLANS.find((p) => digits.startsWith(p.code));
	if (plan === undefined) {
		return e164;
	}
	const national = digits.slice(plan.code.length);
	if (national.length !== plan.groups.reduce((a, b) => a + b, 0)) {
		return e164;
	}
	let at = 0;
	const parts = plan.groups.map((n) => {
		const part = national.slice(at, at + n);
		at += n;
		return part;
	});
	return `+${plan.code} ${parts.join(' ')}`;
}
