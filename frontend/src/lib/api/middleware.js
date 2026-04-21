// handle not ok typical responses such as unauthenticated, renew password and such

import { goto } from '$app/navigation';

/**
 * Are we already sitting on the login page? Used to avoid redirect storms
 * where a 401 response triggers goto('/login/') while we're already there,
 * which can cancel in-flight loads and surface as a flicker/bounce.
 *
 * Reads from window.location (not $page) because this middleware fires
 * from the api client, which is not guaranteed to be inside a Svelte
 * component context.
 */
const isAlreadyOnLogin = () => {
	if (typeof window === 'undefined') return false;
	const p = window.location.pathname;
	return p === '/login' || p === '/login/';
};

/**
 * @param {import("./client").ApiResponse} apiResponse
 * @returns {import("./client").ApiResponse} apiResponse
 **/
export const immediateResponseHandler = (apiResponse) => {
	// Unauthenticated: move the user to the login page.
	//
	// Previously this also called window.location.reload() after goto(),
	// which guaranteed any form state (including unsaved input the user
	// was typing) was blown away whenever a background request returned
	// 401 - a nasty UX hit for anyone whose session ping happened to
	// race with their work. goto() alone is sufficient to get them to
	// /login/; the reload was redundant.
	if (apiResponse.statusCode === 401) {
		if (!isAlreadyOnLogin()) {
			goto('/login/');
		}
		// still return the response so the caller (form handler, etc.)
		// can surface an error message instead of silently hanging
		return apiResponse;
	}
	// If the user must renew their password, redirect to login
	if (apiResponse.statusCode === 400 && apiResponse.error === 'New password required') {
		if (!isAlreadyOnLogin()) {
			goto('/login/');
		}
		return apiResponse;
	}
	return apiResponse;
};
