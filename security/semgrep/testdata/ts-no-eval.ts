function unsafe(code: string): void {
	// ruleid: ts-no-eval
	eval(code);
	// ruleid: ts-no-eval
	const f = new Function('a', code);
	f(1);
	// ruleid: ts-no-eval
	Function(code)();
	// ruleid: ts-no-eval
	setTimeout('run(' + code + ')', 10);
	// ruleid: ts-no-eval
	window.eval(code);
	// ruleid: ts-no-eval
	globalThis.eval(code);
}

function safe(run: () => void): void {
	// ok: ts-no-eval
	setTimeout(run, 10);
	// ok: ts-no-eval
	setTimeout(() => {
		run();
	}, 10);
	// ok: ts-no-eval
	const evaluate = (n: number): number => n + 1;
	evaluate(1);
}

function timers(run: () => void, reject: (e: Error) => void, ms: number): void {
	// ok: ts-no-eval
	setTimeout(() => {
		reject(new Error('timeout'));
	}, ms);
	// ok: ts-no-eval
	setInterval(() => {
		run();
		console.warn('code=tick');
	}, ms);
}

function stringTimers(code: string): void {
	// ruleid: ts-no-eval
	setInterval('tick(' + code + ')', 10);
	// ruleid: ts-no-eval
	setTimeout(`run(${code})`, 10);
}
