// The sheets that are open, oldest first (design doc item 70, `BottomSheet`). Sheets can stack (a
// discard question over an archive sheet) and close in any order, so the page lock follows the
// count, not each sheet: it locks when the first opens and unlocks when the last closes. Only the
// top sheet answers Escape and the focus trap.
export type SheetStackDeps = { lock: () => void; unlock: () => void };

export function createSheetStack<Id>(deps: SheetStackDeps) {
	let open: Id[] = [];
	return {
		push(id: Id): void {
			if (open.length === 0) {
				deps.lock();
			}
			open = [...open.filter((x) => x !== id), id];
		},
		remove(id: Id): void {
			if (!open.includes(id)) {
				return;
			}
			open = open.filter((x) => x !== id);
			if (open.length === 0) {
				deps.unlock();
			}
		},
		isTop(id: Id): boolean {
			return open[open.length - 1] === id;
		}
	};
}
