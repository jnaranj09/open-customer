import { t, type MessageKey } from '../i18n';
import type { PROBLEM_PREFIX, Appointment, Problem } from './schemas';

/** The short name of a problem `type`: `urn:open-customer:problem:overlap` is `overlap`. */
export type ProblemName = Problem['type'] extends `${typeof PROBLEM_PREFIX}${infer Name}`
	? Name
	: never;

export type ApiError =
	| { kind: 'offline' }
	| { kind: 'unauthorized' }
	// `fields` maps a field to the server's message, which is already Spanish and made for her.
	| {
			kind: 'problem';
			status: number;
			type: ProblemName;
			fields: Record<string, string>;
			problem: Problem;
	  }
	| { kind: 'overlap'; overlapping: Appointment[] }
	| { kind: 'response_invalid'; path: string };

export type Result<T> = { ok: true; data: T } | { ok: false; error: ApiError };

// Every problem name has its text; a new name in the spec is a type error here.
const PROBLEM_TEXT: Record<ProblemName, MessageKey> = {
	validation: 'error_validation',
	'not-found': 'error_not_found',
	unauthorized: 'error_unauthorized',
	'invalid-credentials': 'error_invalid_credentials',
	'must-change-password': 'error_must_change_password',
	'forbidden-origin': 'error_forbidden_origin',
	'setup-closed': 'error_setup_closed',
	'setup-token-invalid': 'error_setup_token_invalid',
	overlap: 'error_overlap',
	'duplicate-phone': 'error_duplicate_phone',
	'invalid-transition': 'error_invalid_transition',
	'payload-too-large': 'error_payload_too_large',
	'unsupported-media-type': 'error_unsupported_media_type',
	'rate-limited': 'error_rate_limited',
	internal: 'error_internal'
};

/** The Spanish text for an error. It comes from the problem type, never from the server's `detail`
 *  or `title`. A validation error shows the server's field messages, which are Spanish. */
export function errorText(e: ApiError): string {
	switch (e.kind) {
		case 'offline':
			return t('error_offline');
		case 'unauthorized':
			return t('error_unauthorized');
		case 'overlap':
			return t(PROBLEM_TEXT.overlap);
		case 'response_invalid':
			return t('error_response_invalid');
		case 'problem': {
			const messages = Object.values(e.fields);
			if (e.type === 'validation' && messages.length > 0) {
				return messages.join('. ');
			}
			return t(PROBLEM_TEXT[e.type]);
		}
	}
}
