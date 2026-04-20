import { writable } from 'svelte/store';
import { toast } from 'svelte-sonner';

/**
 * Legacy store kept for backwards compatibility with any code that still
 * reads the raw toast queue directly. The authoritative toast system is now
 * svelte-sonner, so this store is effectively a no-op passthrough that
 * mirrors the most recent toast's metadata for any lingering subscribers.
 *
 * @type {import("svelte/store").Writable<{id: number, text: string, type: string}[]>}
 */
export const toasts = writable([]);

let nextID = 0;

/**
 * Map the legacy type strings used across the codebase to the matching
 * svelte-sonner method. Returns a function that accepts (message, options).
 *
 * @param {"Success"|"Info"|"Warning"|"Error"|string} type
 */
const resolveToast = (type) => {
	switch (type) {
		case 'Success':
			return toast.success;
		case 'Error':
			return toast.error;
		case 'Warning':
			return toast.warning;
		case 'Info':
			return toast.info;
		default:
			return toast;
	}
};

/**
 * Drop-in replacement for the legacy addToast helper. Existing call sites
 * pass (text, type[, visibilityMS]). We forward those to svelte-sonner and
 * also emit a record into the `toasts` store so that any consumer reading
 * the raw store continues to observe activity.
 *
 * @param {string} text
 * @param {"Success"|"Info"|"Warning"|"Error"} type
 * @param {number} [visibilityMS=5000]
 */
export const addToast = (text, type, visibilityMS = 5000) => {
	const id = nextID++;
	const record = { id, text, type };
	const fn = resolveToast(type);
	fn(text, { duration: visibilityMS });

	toasts.update((current) => [...current, record]);
	setTimeout(() => {
		toasts.update((current) => current.filter((t) => t.id !== id));
	}, visibilityMS);
};

/**
 * Removes a toast from the legacy store. svelte-sonner's own dismissal is
 * handled automatically by the <Toaster /> component.
 *
 * @param {number} id
 */
export const removeToast = (id) => {
	toasts.update((current) => current.filter((t) => t.id !== id));
};
