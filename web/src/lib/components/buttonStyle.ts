export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

const BASE =
	'inline-flex min-h-tap items-center justify-center gap-2 rounded-pill px-6 py-3 text-sm font-bold';

const VARIANTS: Record<ButtonVariant, string> = {
	primary: 'bg-accent text-on-accent shadow-raised hover:bg-accent-700 hover:text-on-strong',
	secondary: 'glass-card text-ink hover:bg-glass-strong',
	ghost: 'border border-accent-300 bg-accent-100 text-accent-700 hover:bg-accent-300',
	danger: 'bg-error text-on-strong shadow-raised hover:opacity-90'
};

/** The classes of a button-looking element, shared by `Button` and the links that look like one. */
export function buttonClasses(variant: ButtonVariant): string {
	return `${BASE} ${VARIANTS[variant]}`;
}
