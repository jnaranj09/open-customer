import { describe, expect, it } from 'vitest';
import { createReporter, errorReport } from './report';

class CodedError extends Error {
	code = 'E_BOOM';
}

describe('errorReport', () => {
	it('cuts the stack to 8000', () => {
		const e = new Error('x');
		e.stack = 'a'.repeat(9000);
		expect(errorReport(e, '/', 'v1').stack).toHaveLength(8000);
	});

	it('cuts the stack by characters, not by UTF-16 units', () => {
		const e = new Error('x');
		e.stack = '😀'.repeat(9000);
		expect(Array.from(errorReport(e, '/', 'v1').stack ?? '')).toHaveLength(8000);
	});

	it('uses the error name and code, not a response body', () => {
		const e = new CodedError('{"phone":"+50688887777"}');
		e.name = 'CodedError';
		expect(errorReport(e, '/', 'v1').message).toBe('CodedError E_BOOM');
	});

	it('uses the name alone when there is no code', () => {
		expect(errorReport(new TypeError('Ana Rojas'), '/', 'v1').message).toBe('TypeError');
	});

	it('drops the header lines before the first frame, whatever the message became', () => {
		const e = new Error('Ana Rojas');
		e.stack = 'Error: Ana\nRojas +50688887777\n    at f (app.js:1:1)\n    at g (app.js:2:2)';
		expect(errorReport(e, '/', 'v1').stack).toBe('    at f (app.js:1:1)\n    at g (app.js:2:2)');
	});

	it('keeps a stack that has no header, as Safari and Firefox write it', () => {
		const e = new Error('Ana Rojas');
		e.stack = 'f@app.js:1:1\ng@app.js:2:2';
		expect(errorReport(e, '/', 'v1').stack).toBe('f@app.js:1:1\ng@app.js:2:2');
	});

	it('falls back to Error when the name is empty, so the server accepts it', () => {
		const e = new Error('x');
		e.name = '';
		expect(errorReport(e, '/', 'v1').message).toBe('Error');
	});

	it('reports a thrown non-error by its kind, with no stack', () => {
		expect(errorReport('Ana Rojas', '/', 'v1')).toEqual({
			message: 'non_error',
			stack: null,
			route: '/',
			app_version: 'v1'
		});
	});

	it('cuts the message to 2000, the route to 200 and the version to 64', () => {
		const e = new Error('x');
		e.name = 'N'.repeat(3000);
		const r = errorReport(e, `/${'r'.repeat(300)}`, 'v'.repeat(100));
		expect([r.message.length, r.route.length, r.app_version.length]).toEqual([2000, 200, 64]);
	});

	it('never sends an empty route or version', () => {
		const r = errorReport(new Error('x'), '', '');
		expect([r.route, r.app_version]).toEqual(['/', 'unknown']);
	});
});

describe('createReporter', () => {
	const setup = (now = 0) => {
		const sent: string[] = [];
		const clock = { now };
		const report = createReporter(
			(r) => sent.push(r.message),
			() => clock.now
		);
		return { report, sent, clock };
	};
	const named = (name: string, frame = 'f'): Error =>
		Object.assign(new Error('x'), { name, stack: `${name}: x\n    at ${frame} (app.js:1:1)` });

	it('sends a distinct message once per page load', () => {
		const { report, sent } = setup();
		report(named('A'), '/', 'v1');
		report(named('A'), '/', 'v1');
		expect(sent).toEqual(['A']);
	});

	it('sends the same message again from another place', () => {
		const { report, sent } = setup();
		report(named('A', 'f'), '/', 'v1');
		report(named('A', 'g'), '/', 'v1');
		expect(sent).toEqual(['A', 'A']);
	});

	it('sends different messages', () => {
		const { report, sent } = setup();
		report(named('A'), '/', 'v1');
		report(named('B'), '/', 'v1');
		expect(sent).toEqual(['A', 'B']);
	});

	const burst = (report: ReturnType<typeof setup>['report']): void => {
		for (let i = 0; i < 15; i++) {
			report(named(`E${String(i)}`), '/', 'v1');
		}
	};

	it('sends at most 10 a minute', () => {
		const { report, sent } = setup();
		burst(report);
		expect(sent).toHaveLength(10);
	});

	it('counts again a minute later', () => {
		const { report, sent, clock } = setup();
		burst(report);
		clock.now = 61_000;
		report(named('Later'), '/', 'v1');
		expect(sent).toHaveLength(11);
	});
});
