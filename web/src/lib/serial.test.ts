import { describe, expect, it } from 'vitest';
import { serialLatest } from './serial';

/** A task the test finishes by hand. */
function manual() {
	const log: string[] = [];
	const finish: Array<(v: string) => void> = [];
	const run = serialLatest(
		(): Promise<string> =>
			new Promise((resolve) => {
				log.push('start');
				finish.push((v) => {
					log.push('end ' + v);
					resolve(v);
				});
			})
	);
	return { run, log, finish };
}

describe('serialLatest', () => {
	it('starts the first call at once', () => {
		const { run, log } = manual();
		void run();
		expect(log).toEqual(['start']);
	});

	it('does not start a second call while one runs', () => {
		const { run, log } = manual();
		void run();
		void run();
		expect(log).toEqual(['start']);
	});

	it('runs one call after the running one ends, and the older answer ends first', async () => {
		const { run, log, finish } = manual();
		const first = run();
		const second = run();
		finish[0]?.('a');
		await first;
		await Promise.resolve();
		await Promise.resolve();
		finish[1]?.('b');
		expect([await second, log]).toEqual(['b', ['start', 'end a', 'start', 'end b']]);
	});

	it('gives every call made while one runs the same follow-up run', async () => {
		const { run, log, finish } = manual();
		const first = run();
		const second = run();
		const third = run();
		finish[0]?.('a');
		await first;
		await Promise.resolve();
		await Promise.resolve();
		finish[1]?.('b');
		expect([await second, await third, log.filter((l) => l === 'start').length]).toEqual([
			'b',
			'b',
			2
		]);
	});

	it('starts a fresh run for a call made after everything ended', async () => {
		const { run, log, finish } = manual();
		const first = run();
		finish[0]?.('a');
		await first;
		void run();
		expect(log).toEqual(['start', 'end a', 'start']);
	});

	it('still runs the follow-up when the running one rejects', async () => {
		let calls = 0;
		const run = serialLatest(() => {
			calls += 1;
			return calls === 1 ? Promise.reject(new Error('boom')) : Promise.resolve('ok');
		});
		const first = run();
		const second = run();
		await expect(first).rejects.toThrow('boom');
		expect(await second).toBe('ok');
	});
});
