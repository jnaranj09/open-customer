import { describe, expect, it } from 'vitest';
import { WIDE_MIN_PX, layout } from './layout';

describe('layout', () => {
	it('switches to wide at 1024 px', () => {
		expect(WIDE_MIN_PX).toBe(1024);
	});

	it('is compact where there is no window to measure (the phone layout is the safe default)', () => {
		expect(layout.current).toBe('compact');
	});
});
