import { describe, expect, test } from "bun:test";
import { EventBuffer } from "./event-buffer";

describe("EventBuffer", () => {
	test("keeps only the newest events in chronological order", () => {
		const buffer = new EventBuffer(3);
		for (const message of ["one", "two", "three", "four"]) {
			buffer.push({
				timestamp: `2026-07-08T00:00:0${message.length}.000Z`,
				type: "deploy",
				message,
			});
		}

		expect(buffer.size).toBe(3);
		expect(buffer.getAll().map(({ message }) => message)).toEqual([
			"two",
			"three",
			"four",
		]);
	});

	test("adds timestamps and does not expose mutable internal state", () => {
		const buffer = new EventBuffer();
		const stored = buffer.add("scale", "Scaled api to 3 replicas");
		const snapshot = buffer.getAll();
		snapshot[0].message = "changed";
		snapshot.push({ timestamp: "now", type: "restart", message: "extra" });

		expect(Date.parse(stored.timestamp)).not.toBeNaN();
		expect(buffer.size).toBe(1);
		expect(buffer.getAll()[0].message).toBe("Scaled api to 3 replicas");
	});

	test("rejects invalid capacities", () => {
		expect(() => new EventBuffer(0)).toThrow(RangeError);
		expect(() => new EventBuffer(1.5)).toThrow(RangeError);
	});
});
