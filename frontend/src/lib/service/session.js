import { API } from '$lib/api/api.js';
import { AppStateService } from './appState';

/**
 * Session class
 *
 * Use Session.instance to get the default global singleton instance
 * The first time you call Session.instance or the constructor, it will be initialized with the global api client
 */
export class Session {
	/**
	 * Global singleton session instance
	 * @type {Session|null}
	 */
	static #_instance = null;

	static get instance() {
		if (!Session.#_instance) {
			Session.#_instance = new Session();
		}
		return Session.#_instance;
	}

	/**
	 * The interval in milliseconds between each session ping
	 *
	 * @type {number}
	 */
	#intervalMS = 1000 * 60;

	/**
	 * @type {API|null}
	 */
	#apiClient = null;

	/**
	 * @type {AppStateService|null}
	 */
	#appStateService = null;

	/**
	 * @type {number|null}
	 */
	#intervalID = null;

	/**
	 * @type {boolean}
	 */
	#isRunning = false;

	get isRunning() {
		return this.#isRunning;
	}

	/**
	 * Consecutive transient (network / 5xx) ping failures. Reset on success.
	 * Only after crossing the threshold do we treat the session as logged
	 * out - a single flaky ping (Wi-Fi hiccup, brief backend blip) should
	 * not kick the user to /login.
	 * @type {number}
	 */
	#consecutiveFailures = 0;

	/**
	 * Number of consecutive transient failures tolerated before giving up
	 * on the session. 401 responses bypass this and log out immediately.
	 * @type {number}
	 */
	#failureThreshold = 3;

	/**
	 * @type {boolean}
	 */
	#debug = false;

	/**
	 * If no client is provided, use it automatically uses the global api client
	 * @param {API} apiClient
	 * @param {AppStateService} appStateService
	 */
	constructor(apiClient = API.instance, appStateService = AppStateService.instance) {
		this.#apiClient = apiClient;
		this.#appStateService = appStateService;
	}

	/**
	 * log to console if debug is enabled
	 * @param {...*} x
	 */
	#log(...x) {
		if (this.#debug) {
			console.log('session:', ...x);
		}
	}

	/**
	 * ping session
	 *
	 * Failure policy:
	 *  - HTTP 401 (or error === 'unauthorized'): server has explicitly
	 *    rejected the session; log out immediately.
	 *  - Network error / 5xx / other non-success: transient; increment
	 *    the consecutive-failure counter and only log out once the
	 *    threshold is crossed.
	 *  - Success: reset the counter.
	 *
	 * @throws {Error} if session ping throws synchronously
	 */
	async ping() {
		this.#log('pinging...');
		let sessionPingResult;
		try {
			sessionPingResult = await this.#apiClient.session.ping();
		} catch (e) {
			// TypeError from fetch = network unreachable; always transient
			this.#consecutiveFailures += 1;
			this.#log(
				'ping threw (transient)',
				e,
				'failures=',
				this.#consecutiveFailures
			);
			if (this.#consecutiveFailures >= this.#failureThreshold) {
				this.#appStateService.setLogin(AppStateService.LOGIN.LOGGED_OUT);
			}
			return;
		}
		if (!sessionPingResult.success) {
			const status = sessionPingResult.statusCode;
			const err = sessionPingResult.error;
			const isAuthFailure =
				status === 401 ||
				err === 'unauthorized' ||
				err === 'Unauthorized';
			if (isAuthFailure) {
				// explicit server rejection - no tolerance
				this.#consecutiveFailures = 0;
				this.#appStateService.setLogin(AppStateService.LOGIN.LOGGED_OUT);
				return;
			}
			// transient (5xx, 0, network-translated): tolerate a few in a row
			this.#consecutiveFailures += 1;
			this.#log(
				'ping failed (transient)',
				status,
				err,
				'failures=',
				this.#consecutiveFailures
			);
			if (this.#consecutiveFailures >= this.#failureThreshold) {
				this.#appStateService.setLogin(AppStateService.LOGIN.LOGGED_OUT);
			}
			return;
		}
		// success - clear the transient-failure counter
		this.#consecutiveFailures = 0;
		// user is logged in
		this.#appStateService.setLogin(AppStateService.LOGIN.LOGGED_IN, {
			name: sessionPingResult.data.name,
			username: sessionPingResult.data.username,
			company: sessionPingResult.data.company,
			role: sessionPingResult.data.role
		});
		// check if app is installed
		this.#log('user is logged in - retrieving install status');
		const res = await this.#apiClient.option.get('is_installed');
		if (res.data.value === 'true') {
			this.#appStateService.setIsInstalled();
		} else {
			this.#appStateService.setIsNotInstalled();
		}
		this.#log('ping success');
	}

	debugOn() {
		this.#debug = true;
	}

	debugOff() {
		this.#debug = false;
	}

	/**
	 * start session ping
	 *
	 * @throws {Error} if session is already started
	 * @throws {Error} if session initialization failed
	 */
	async start() {
		if (this.#isRunning) {
			this.#log('already started');
			throw new Error('session is already started');
		}
		this.#isRunning = true;
		this.#log('initial ping');
		try {
			// initial ping
			// setup continous ping
			this.#intervalID = window.setInterval(async () => {
				try {
					await this.ping();
				} catch (e) {
					this.#log('ping failed', e);
				}
			}, this.#intervalMS);
			await this.ping();
			this.#log('ping success');
			this.#log('continous ping is running');
		} catch (e) {
			this.#isRunning = false;
			this.#log('initial ping failed', e);
			throw e;
		}
	}

	/**
	 * stop session ping
	 *
	 * @throws {Error} if session is not started
	 */
	stop = () => {
		if (!this.#isRunning) {
			this.#log('not started');
			throw new Error('session is not started');
		}
		clearInterval(this.#intervalID);
		this.#isRunning = false;
		this.#log('stopped');
	};
}
