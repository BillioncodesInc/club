/**
 * Represents the response object returned by the API functions.
 * @typedef {Object} ApiResponse
 * @property {boolean} success - Indicates whether the request was successful.
 * @property {number} statusCode - The status code of the response.
 * @property {string} error - The error message, if any.
 * @property {any} data - The data returned by the request.
 */

/**
 * Default timeout for API requests (in milliseconds).
 * Standard requests use 30 seconds, extended operations use 3.5 minutes.
 */
const DEFAULT_TIMEOUT = 30000;
export const EXTENDED_TIMEOUT = 210000; // 3.5 minutes for browser automation operations

/**
 * Creates a fetch request with an AbortController timeout.
 * @param {string} url - The URL to fetch.
 * @param {Object} options - Fetch options.
 * @param {number} [timeout] - Timeout in milliseconds.
 * @returns {Promise<Response>} - The fetch response.
 */
const fetchWithTimeout = async (url, options, timeout = DEFAULT_TIMEOUT) => {
	const controller = new AbortController();
	const timeoutId = setTimeout(() => controller.abort(), timeout);
	try {
		const response = await fetch(url, {
			...options,
			signal: controller.signal
		});
		clearTimeout(timeoutId);
		return response;
	} catch (error) {
		clearTimeout(timeoutId);
		if (error.name === 'AbortError') {
			throw new Error('Request timed out. The server may still be processing your request.');
		}
		throw error;
	}
};

/**
 * Fetches JSON data from the specified URL using the GET method.
 * @param {string} url - The URL to fetch the JSON data from.
 * @param {number} [timeout] - Optional timeout in milliseconds.
 * @returns {Promise<Object>} - A promise that resolves to the response object containing the JSON data.
 */
export const getJSON = async (url, timeout) => {
	const res = await fetchWithTimeout(url, {
		method: 'GET'
	}, timeout);
	const body = await res.json();
	return newResponse(body.success, res.status, body.error, body.data);
};

/**
 * Sends JSON data to the specified URL using the POST method.
 * @param {string} url - The URL to send the JSON data to.
 * @param {Object} data - The JSON data to send.
 * @returns {Promise<Object>} - A promise that resolves to the response object containing the JSON data.
 */
export const postJSON = async (url, data, timeout) => {
	const res = await fetchWithTimeout(url, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json'
		},
		body: JSON.stringify(data)
	}, timeout);
	let body = {};
	try {
		body = await res.json();
	} catch (e) {
		body = {
			success: false,
			error: 'invalid JSON in response'
		};
	}
	return newResponse(body.success, res.status, body.error, body.data);
};

/**
 * Sends JSON data to the specified URL using the POST method.
 * @param {string} url - The URL to send the JSON data to.
 * @param {Object} data - The JSON data to send.
 * @returns {Promise<Object>} - A promise that resolves to the response object containing the JSON data.
 */
export const patchJSON = async (url, data, timeout) => {
	const res = await fetchWithTimeout(url, {
		method: 'PATCH',
		headers: {
			'Content-Type': 'application/json'
		},
		body: JSON.stringify(data)
	}, timeout);
	let body = {};
	try {
		body = await res.json();
	} catch (e) {
		body = {
			success: false,
			error: 'invalid JSON in response'
		};
	}
	return newResponse(body.success, res.status, body.error, body.data);
};

/**
 * Sends JSON data to the specified URL using the PUT method.
 * @param {string} url - The URL to send the JSON data to.
 * @param {Object} data - The JSON data to send.
 * @returns {Promise<Object>} - A promise that resolves to the response object containing the JSON data.
 */
export const putJSON = async (url, data, timeout) => {
	const res = await fetchWithTimeout(url, {
		method: 'PUT',
		headers: {
			'Content-Type': 'application/json'
		},
		body: JSON.stringify(data)
	}, timeout);
	let body = {};
	try {
		body = await res.json();
	} catch (e) {
		body = {
			success: false,
			error: 'invalid JSON in response'
		};
	}
	return newResponse(body.success, res.status, body.error, body.data);
};

/**
 * Sends JSON data to the specified URL using the DELETE method.
 * @param {string} url - The URL to send the JSON data to.
 * @param {Object} data - The JSON data to send.
 * @returns {Promise<Object>} - A promise that resolves to the response object containing the JSON data.
 */
export const deleteJSON = async (url, data, timeout) => {
	const res = await fetchWithTimeout(url, {
		method: 'DELETE',
		headers: {
			'Content-Type': 'application/json'
		},
		body: JSON.stringify(data)
	}, timeout);
	let body = {};
	try {
		body = await res.json();
	} catch (e) {
		body = {
			success: false,
			error: 'invalid JSON in response',
			data: null
		};
	}
	return newResponse(body.success, res.status, body.error, body.data);
};

/**
 * Sends multipart form data to the specified URL using the POST method.
 * @param {string} url - The URL to send the multipart data to.
 * @param {FormData} formData - The FormData object containing the data to send.
 * @returns {Promise<Object>} - A promise that resolves to the response object containing the JSON data.
 */
export const postMultipart = async (url, formData, timeout) => {
	console.log(formData);
	const res = await fetchWithTimeout(url, {
		method: 'POST',
		body: formData
	}, timeout);
	let body = {};
	try {
		body = await res.json();
	} catch (e) {
		body = {
			success: false,
			error: 'invalid JSON in response'
		};
	}
	return newResponse(body.success, res.status, body.error, body.data);
};

/**
 * Sends a DELETE request to the specified URL.
 * @param {string} url - The URL to send the DELETE request to.
 * @returns {Promise<Object>} - A promise that resolves to the response object containing the JSON data.
 */
export const deleteReq = async (url, timeout) => {
	// Function implementation
	const res = await fetchWithTimeout(url, {
		method: 'DELETE'
	}, timeout);
	try {
		const body = await res.json();
		return newResponse(body.success, res.status, body.error, body.data);
	} catch (e) {
		return newResponse(false, res.status, 'invalid JSON in response', null);
	}
};

/**
 * Creates a new response object.
 * @param {boolean} success - Indicates whether the request was successful.
 * @param {number} statusCode - The status code of the response.
 * @param {string} error - The error message, if any.
 * @param {any} data - The data returned by the request.
 * @returns {Object} - The response object.
 */
export function newResponse(success, statusCode, error, data) {
	return {
		success: success,
		statusCode: statusCode,
		error: error,
		data: data
	};
}
