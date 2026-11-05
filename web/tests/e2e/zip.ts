// Reads the export ZIP (plan D7) without a zip dependency: it walks the local file headers and
// inflates each entry with Node's zlib. The export streams its entries, so each one has a data
// descriptor and no sizes in its header; a deflated entry ends where its deflate stream ends.
import { inflateRawSync } from 'node:zlib';

const LOCAL_HEADER = 0x04034b50;
const CENTRAL_HEADER = 0x02014b50;
const DATA_DESCRIPTOR = 0x08074b50;
const HAS_DESCRIPTOR = 0x0008;
const STORED = 0;
const DEFLATED = 8;

/** The size of the data descriptor at `at`: CRC and two 32-bit sizes, with an optional
 *  signature. A ZIP64 descriptor (64-bit sizes) is not read. */
function descriptorSize(buf: Buffer, at: number): number {
	return buf.readUInt32LE(at) === DATA_DESCRIPTOR ? 16 : 12;
}

/** Every file of the ZIP in `buf`, by name, as UTF-8 text. Throws on a compression method other
 *  than stored or deflate, and on a stored entry with a data descriptor (it cannot be sized). */
export function readZip(buf: Buffer): Map<string, string> {
	const files = new Map<string, string>();
	let at = 0;
	while (at + 4 <= buf.length && buf.readUInt32LE(at) === LOCAL_HEADER) {
		const flags = buf.readUInt16LE(at + 6);
		const method = buf.readUInt16LE(at + 8);
		const size = buf.readUInt32LE(at + 18);
		const nameLength = buf.readUInt16LE(at + 26);
		const extraLength = buf.readUInt16LE(at + 28);
		const name = buf.toString('utf8', at + 30, at + 30 + nameLength);
		const start = at + 30 + nameLength + extraLength;
		const described = (flags & HAS_DESCRIPTOR) !== 0;

		let data: Buffer;
		let used: number;
		if (method === DEFLATED) {
			// `info` gives the engine, whose `bytesWritten` is the compressed bytes it consumed.
			const { buffer, engine } = inflateRawSync(buf.subarray(start), { info: true }) as unknown as {
				buffer: unknown;
				engine?: { bytesWritten?: unknown };
			};
			if (!Buffer.isBuffer(buffer) || typeof engine?.bytesWritten !== 'number') {
				throw new Error(`readZip: ${name}: zlib did not report the bytes it consumed`);
			}
			data = buffer;
			used = engine.bytesWritten;
		} else if (method === STORED && !described) {
			data = buf.subarray(start, start + size);
			used = size;
		} else {
			throw new Error(`readZip: ${name}: method ${String(method)} cannot be read`);
		}
		files.set(name, data.toString('utf8'));
		at = start + used;
		if (described) {
			at += descriptorSize(buf, at);
		}
	}
	if (files.size === 0 || at + 4 > buf.length || buf.readUInt32LE(at) !== CENTRAL_HEADER) {
		throw new Error('readZip: not a ZIP, or an entry it could not walk past');
	}
	return files;
}
