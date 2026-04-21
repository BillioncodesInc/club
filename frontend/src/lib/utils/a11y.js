/**
 * a11y helpers — tiny utilities to normalize keyboard handling for
 * non-native buttons (elements using role="button").
 */

/**
 * Invoke `handler` when Enter or Space is pressed on a non-button element
 * that is acting as a button (role="button").
 *
 * Usage:
 *   <div role="button" tabindex="0"
 *        on:click={doThing}
 *        on:keydown={(e) => buttonRoleKeydown(e, doThing)}>
 *
 * @param {KeyboardEvent} e
 * @param {(e: KeyboardEvent) => void} handler
 */
export function buttonRoleKeydown(e, handler) {
	if (e.key === 'Enter' || e.key === ' ' || e.key === 'Spacebar') {
		e.preventDefault();
		handler(e);
	}
}
