/**
 * Thin shim that wraps svelte-sonner's toast API so new code can call a
 * single entry point while existing code continues to use the store-based
 * addToast helper at `$lib/store/toast`. Both layers converge on svelte-sonner.
 */
import { toast } from 'svelte-sonner';
import { addToast as legacyAddToast } from '$lib/store/toast';

export { toast };

/**
 * Drop-in replacement for the legacy addToast(text, type[, ms]) signature.
 * Delegates to the store-level helper so both code paths stay in sync.
 *
 * @param {string} text
 * @param {"Success"|"Info"|"Warning"|"Error"} type
 * @param {number} [visibilityMS=5000]
 */
export const addToast = (text, type, visibilityMS = 5000) => {
	legacyAddToast(text, type, visibilityMS);
};

/**
 * Direct typed helpers for new call sites that prefer an explicit API.
 */
export const toastSuccess = (text, options) => toast.success(text, options);
export const toastError = (text, options) => toast.error(text, options);
export const toastWarning = (text, options) => toast.warning(text, options);
export const toastInfo = (text, options) => toast.info(text, options);
