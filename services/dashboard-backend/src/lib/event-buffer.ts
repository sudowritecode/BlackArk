export interface ActivityEvent {
	timestamp: string;
	type: string;
	message: string;
	[key: string]: unknown;
}

export interface NewActivityEvent {
	timestamp?: string | Date;
	type: string;
	message: string;
	[key: string]: unknown;
}

export const DEFAULT_EVENT_BUFFER_CAPACITY = 100;

/** A bounded, synchronous event store. Returned events are defensive copies. */
export class EventBuffer {
	readonly capacity: number;
	#events: ActivityEvent[] = [];

	constructor(capacity = DEFAULT_EVENT_BUFFER_CAPACITY) {
		if (!Number.isInteger(capacity) || capacity < 1) {
			throw new RangeError("event buffer capacity must be a positive integer");
		}
		this.capacity = capacity;
	}

	get size(): number {
		return this.#events.length;
	}

	push(event: NewActivityEvent): ActivityEvent {
		const stored: ActivityEvent = {
			...event,
			timestamp:
				event.timestamp instanceof Date
					? event.timestamp.toISOString()
					: (event.timestamp ?? new Date().toISOString()),
		};
		this.#events.push(stored);
		if (this.#events.length > this.capacity) {
			this.#events.splice(0, this.#events.length - this.capacity);
		}
		return { ...stored };
	}

	add(type: string, message: string): ActivityEvent {
		return this.push({ type, message });
	}

	getAll(): ActivityEvent[] {
		return this.#events.map((event) => ({ ...event }));
	}

	clear(): void {
		this.#events = [];
	}
}

export const eventBuffer = new EventBuffer();
