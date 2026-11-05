/** The surface of a form field (the .pen TextField's input and SelectField): the strong glass
 *  fill, the small radius and the accent edge, red when the value is invalid. Shared by
 *  `TextField`, `SelectField` and the fields that only look like one (the attribute picker, the
 *  template editor, the icon file input). The caller adds size, padding and text color. */
export function fieldClasses(invalid: boolean): string {
	return `rounded-sm border bg-glass-strong text-base ${invalid ? 'border-error' : 'border-accent'}`;
}
