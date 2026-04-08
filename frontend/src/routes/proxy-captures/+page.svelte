<script>
	import { api } from '$lib/api/apiProxy.js';
	import { onMount } from 'svelte';
	import { addToast } from '$lib/store/toast';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Table from '$lib/components/table/Table.svelte';
	import TableRow from '$lib/components/table/TableRow.svelte';
	import TableCell from '$lib/components/table/TableCell.svelte';
	import TableCellEmpty from '$lib/components/table/TableCellEmpty.svelte';
	import TableCellAction from '$lib/components/table/TableCellAction.svelte';
	import TableDeleteButton from '$lib/components/table/TableDeleteButton2.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { newTableURLParams } from '$lib/service/tableURLParams';
	import TableDropDownEllipsis from '$lib/components/table/TableDropDownEllipsis.svelte';
	import DeleteAlert from '$lib/components/modal/DeleteAlert.svelte';
	import BigButton from '$lib/components/BigButton.svelte';
	import { showIsLoading, hideIsLoading } from '$lib/store/loading';

	// local state
	let captures = [];
	let capturesHasNextPage = false;
	const tableURLParams = newTableURLParams();
	let isTableLoading = false;
	let isDeleteAlertVisible = false;
	let isDeleteAllAlertVisible = false;
	let deleteValues = { id: null, ip: null };
	let expandedRow = null;

	// Filter state
	let activeFilter = 'all'; // 'all', 'credentials', 'cookies'

	const refreshCaptures = async () => {
		try {
			isTableLoading = true;
			const params = new URLSearchParams({
				page: tableURLParams.currentPage,
				perPage: tableURLParams.perPage,
				sortBy: tableURLParams.sortBy || 'created_at',
				sortOrder: tableURLParams.sortOrder || 'desc',
				search: tableURLParams.search || '',
				filter: activeFilter
			});
			const res = await api.proxyCaptures.getAll(params.toString());
			if (res.success) {
				captures = res.data.rows || [];
				capturesHasNextPage = res.data.hasNextPage || false;
				return;
			}
			throw res.error;
		} catch (e) {
			addToast('Failed to load proxy captures', 'Error');
			console.error('failed to load proxy captures', e);
		} finally {
			isTableLoading = false;
		}
	};

	const setFilter = (filter) => {
		activeFilter = filter;
		tableURLParams.currentPage = 1;
		refreshCaptures();
	};

	onMount(() => {
		refreshCaptures();
		tableURLParams.onChange(refreshCaptures);
		return () => {
			tableURLParams.unsubscribe();
		};
	});

	const openDeleteAlert = (capture) => {
		isDeleteAlertVisible = true;
		deleteValues.id = capture.ID;
		deleteValues.ip = capture.IPAddress;
	};

	const openDeleteAllAlert = () => {
		isDeleteAllAlertVisible = true;
	};

	const deleteCapture = async (id) => {
		const action = api.proxyCaptures.deleteByID(id);
		action
			.then((res) => {
				if (res.success) {
					refreshCaptures();
					return;
				}
				throw res.error;
			})
			.catch((e) => {
				addToast('Failed to delete capture', 'Error');
				console.error('failed to delete capture', e);
			});
		return action;
	};

	const deleteAllCaptures = async () => {
		const action = api.proxyCaptures.deleteAll();
		action
			.then((res) => {
				if (res.success) {
					refreshCaptures();
					return;
				}
				throw res.error;
			})
			.catch((e) => {
				addToast('Failed to delete all captures', 'Error');
				console.error('failed to delete all captures', e);
			});
		return action;
	};

	const toggleExpand = (id) => {
		expandedRow = expandedRow === id ? null : id;
	};

	const formatDate = (dateStr) => {
		if (!dateStr) return '';
		const d = new Date(dateStr);
		return d.toLocaleString();
	};

	const maskPassword = (pw) => {
		if (!pw) return '';
		if (pw.length <= 4) return '****';
		return pw.substring(0, 2) + '****' + pw.substring(pw.length - 2);
	};

	let showPasswords = {};
	const togglePassword = (id) => {
		showPasswords[id] = !showPasswords[id];
		showPasswords = showPasswords;
	};

	const copyToClipboard = (text) => {
		navigator.clipboard.writeText(text).then(() => {
			addToast('Copied to clipboard', 'Success');
		}).catch(() => {
			addToast('Failed to copy', 'Error');
		});
	};

	// --- Cookie Format Helpers ---

	/**
	 * Parse raw cookies from proxy capture and restore original domains.
	 * Returns an array of cookie objects with proper domain restoration.
	 */
	const parseCookies = (cookiesStr) => {
		try {
			const cookies = JSON.parse(cookiesStr);
			if (!Array.isArray(cookies)) return [];
			return cookies;
		} catch {
			return [];
		}
	};

	/**
	 * Restore the real domain from original_host field when present.
	 */
	const getEffectiveDomain = (cookie) => {
		if (cookie.original_host) return cookie.original_host;
		return cookie.domain || '';
	};

	/**
	 * Convert proxy capture cookies to browser extension format (Cookie Editor / EditThisCookie).
	 * This is the JSON format that browser extensions can import.
	 */
	const toBrowserExtensionFormat = (cookiesStr) => {
		const cookies = parseCookies(cookiesStr);
		return cookies.map(c => {
			const domain = getEffectiveDomain(c);
			const hostOnly = domain && !domain.startsWith('.');
			let sameSite = 'no_restriction';
			if (c.sameSite) {
				switch (c.sameSite.toLowerCase()) {
					case 'strict': sameSite = 'strict'; break;
					case 'lax': sameSite = 'lax'; break;
					case 'none': sameSite = 'no_restriction'; break;
					default: sameSite = 'no_restriction';
				}
			}
			const cookie = {
				domain: domain,
				hostOnly: hostOnly,
				httpOnly: c.httpOnly === 'true' || c.httpOnly === true,
				name: c.name || '',
				path: c.path || '/',
				sameSite: sameSite,
				secure: c.secure === 'true' || c.secure === true,
				session: !c.expires && !c.maxAge,
				storeId: '0',
				value: c.value || ''
			};
			if (c.expires) {
				const expireDate = new Date(c.expires);
				if (!isNaN(expireDate.getTime())) {
					cookie.expirationDate = expireDate.getTime() / 1000;
					cookie.session = false;
				}
			} else if (c.maxAge) {
				const maxAgeSeconds = parseInt(c.maxAge);
				if (!isNaN(maxAgeSeconds)) {
					cookie.expirationDate = Date.now() / 1000 + maxAgeSeconds;
					cookie.session = false;
				}
			}
			return cookie;
		});
	};

	/**
	 * Format cookies as Netscape/Mozilla cookie file format.
	 */
	const toNetscapeFormat = (cookiesStr) => {
		const cookies = parseCookies(cookiesStr);
		let lines = [
			'# Netscape HTTP Cookie File',
			'# Generated by Phishing Club',
			'# https://curl.se/docs/http-cookies.html',
			''
		];
		for (const c of cookies) {
			const domain = getEffectiveDomain(c);
			const includeSubdomains = domain.startsWith('.') ? 'TRUE' : 'FALSE';
			const path = c.path || '/';
			const secure = (c.secure === 'true' || c.secure === true) ? 'TRUE' : 'FALSE';
			let expiry = '0';
			if (c.expires) {
				const d = new Date(c.expires);
				if (!isNaN(d.getTime())) expiry = Math.floor(d.getTime() / 1000).toString();
			}
			const httpOnly = (c.httpOnly === 'true' || c.httpOnly === true) ? '#HttpOnly_' : '';
			lines.push(`${httpOnly}${domain}\t${includeSubdomains}\t${path}\t${secure}\t${expiry}\t${c.name || ''}\t${c.value || ''}`);
		}
		return lines.join('\n');
	};

	/**
	 * Format cookies as a Cookie header string (name=value; name2=value2).
	 */
	const toHeaderFormat = (cookiesStr) => {
		const cookies = parseCookies(cookiesStr);
		return cookies
			.filter(c => c.name && c.value)
			.map(c => `${c.name}=${c.value}`)
			.join('; ');
	};

	/**
	 * Format cookies as JavaScript console commands.
	 */
	const toConsoleFormat = (cookiesStr) => {
		const cookies = parseCookies(cookiesStr);
		let lines = [
			'// Paste this in the browser console on the target domain',
			'// Generated by Phishing Club',
			''
		];
		for (const c of cookies) {
			const domain = getEffectiveDomain(c);
			const path = c.path || '/';
			let expiry = '';
			if (c.expires) {
				const d = new Date(c.expires);
				if (!isNaN(d.getTime())) expiry = `; expires=${d.toUTCString()}`;
			}
			const secure = (c.secure === 'true' || c.secure === true) ? '; Secure' : '';
			const sameSite = c.sameSite && c.sameSite !== 'None' ? `; SameSite=${c.sameSite}` : '';
			lines.push(`document.cookie = "${c.name || ''}=${c.value || ''}; domain=${domain}; path=${path}${expiry}${secure}${sameSite}";`);
		}
		return lines.join('\n');
	};

	/**
	 * Copy cookies in a specific format.
	 */
	const copyCookiesAs = (cookiesStr, format) => {
		let text = '';
		let formatName = '';
		switch (format) {
			case 'json':
				text = JSON.stringify(toBrowserExtensionFormat(cookiesStr), null, 2);
				formatName = 'JSON (Browser Extension)';
				break;
			case 'netscape':
				text = toNetscapeFormat(cookiesStr);
				formatName = 'Netscape';
				break;
			case 'header':
				text = toHeaderFormat(cookiesStr);
				formatName = 'Cookie Header';
				break;
			case 'console':
				text = toConsoleFormat(cookiesStr);
				formatName = 'Console';
				break;
			case 'raw':
			default:
				try {
					text = JSON.stringify(JSON.parse(cookiesStr), null, 2);
				} catch {
					text = cookiesStr;
				}
				formatName = 'Raw JSON';
				break;
		}
		copyToClipboard(text);
		addToast(`Copied as ${formatName}`, 'Success');
	};

	const sendToCookieStore = async (capture) => {
		if (!capture.Cookies) {
			addToast('No cookies to send', 'Error');
			return;
		}
		const name = capture.Username
			? `Proxy: ${capture.Username}`
			: `Proxy: ${capture.IPAddress || 'Unknown'}`;
		showIsLoading();
		try {
			const res = await api.cookieStore.importFromCapture(
				capture.ID,
				name,
				capture.Cookies
			);
			if (res.success) {
				addToast('Cookies sent to Cookie Store. Validating session...', 'Success');
			} else {
				throw res.error;
			}
		} catch (e) {
			addToast('Failed to send to Cookie Store: ' + (e || ''), 'Error');
			console.error('failed to send to cookie store', e);
		} finally {
			hideIsLoading();
		}
	};

	// Cookie format dropdown state
	let cookieFormatDropdown = null;
	const toggleCookieFormatDropdown = (captureId) => {
		cookieFormatDropdown = cookieFormatDropdown === captureId ? null : captureId;
	};

	const getCookieCount = (cookiesStr) => {
		try {
			const parsed = JSON.parse(cookiesStr);
			if (Array.isArray(parsed)) return parsed.length;
			return 0;
		} catch {
			return 0;
		}
	};

	const getCookieDomains = (cookiesStr) => {
		try {
			const parsed = JSON.parse(cookiesStr);
			if (!Array.isArray(parsed)) return [];
			const domains = new Set();
			for (const c of parsed) {
				const d = c.original_host || c.domain || '';
				if (d) {
					// Normalize: remove leading dot
					domains.add(d.replace(/^\./, ''));
				}
			}
			return [...domains].sort();
		} catch {
			return [];
		}
	};
</script>

<HeadTitle title="Proxy Captures" />

<main>
	<Headline>Proxy Captures</Headline>
	<p style="margin-bottom: 1rem; opacity: 0.7;">
		Credentials and cookies captured from direct proxy visits (without a campaign link).
	</p>

	<div class="controls-row">
		<div class="filter-group">
			<button
				class="filter-btn"
				class:active={activeFilter === 'all'}
				on:click={() => setFilter('all')}
			>
				All
			</button>
			<button
				class="filter-btn"
				class:active={activeFilter === 'credentials'}
				on:click={() => setFilter('credentials')}
			>
				With Credentials
			</button>
			<button
				class="filter-btn"
				class:active={activeFilter === 'cookies'}
				on:click={() => setFilter('cookies')}
			>
				Cookies Only
			</button>
		</div>
		<BigButton on:click={openDeleteAllAlert}>Delete all captures</BigButton>
	</div>

	<Table
		columns={[
			{ column: 'Time', size: 'small' },
			{ column: 'IP Address', size: 'small' },
			{ column: 'Username', size: 'medium' },
			{ column: 'Password', size: 'small' },
			{ column: 'Cookies', size: 'small' },
			{ column: 'Domain', size: 'small' }
		]}
		sortable={['Time', 'IP Address', 'Username']}
		hasData={!!captures.length}
		hasNextPage={capturesHasNextPage}
		plural="Captures"
		pagination={tableURLParams}
		isGhost={isTableLoading}
	>
		{#each captures as capture}
			<TableRow>
				<TableCell value={formatDate(capture.CreatedAt)} />
				<TableCell value={capture.IPAddress || ''} />
				<TableCell>
					{#if capture.Username}
						<span class="credential-badge">{capture.Username}</span>
					{:else}
						<span>-</span>
					{/if}
				</TableCell>
				<TableCell>
					{#if capture.Password}
						<span style="display: flex; align-items: center; gap: 0.5rem;">
							<code>{showPasswords[capture.ID] ? capture.Password : maskPassword(capture.Password)}</code>
							<button
								class="small-btn"
								on:click|stopPropagation={() => togglePassword(capture.ID)}
								title={showPasswords[capture.ID] ? 'Hide' : 'Show'}
							>
								{showPasswords[capture.ID] ? 'Hide' : 'Show'}
							</button>
							<button
								class="small-btn"
								on:click|stopPropagation={() => copyToClipboard(capture.Password)}
								title="Copy"
							>
								Copy
							</button>
						</span>
					{:else}
						<span>-</span>
					{/if}
				</TableCell>
				<TableCell>
				{#if capture.Cookies}
					<span style="display: flex; align-items: center; gap: 0.5rem;">
						<span class="cookie-badge">
							{getCookieCount(capture.Cookies)} cookies
						</span>
					</span>
				{:else}
					<span>-</span>
				{/if}
				</TableCell>
				<TableCell value={capture.TargetDomain || capture.PhishDomain || ''} />
				<TableCellEmpty />
				<TableCellAction>
					<TableDropDownEllipsis>
						<button class="dropdown-item" on:click={() => toggleExpand(capture.ID)}>
							<svg class="w-4 h-4 mr-2 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
							{expandedRow === capture.ID ? 'Collapse' : 'Details'}
						</button>
						{#if capture.Cookies}
							<div class="dropdown-divider"></div>
							<div class="dropdown-section-label">Copy Cookies As</div>
							<button class="dropdown-item" on:click={() => copyCookiesAs(capture.Cookies, 'json')}>
								<svg class="w-4 h-4 mr-2 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"/></svg>
								JSON (Browser Extension)
							</button>
							<button class="dropdown-item" on:click={() => copyCookiesAs(capture.Cookies, 'netscape')}>
								<svg class="w-4 h-4 mr-2 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>
								Netscape (cookies.txt)
							</button>
							<button class="dropdown-item" on:click={() => copyCookiesAs(capture.Cookies, 'header')}>
								<svg class="w-4 h-4 mr-2 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"/></svg>
								Cookie Header
							</button>
							<button class="dropdown-item" on:click={() => copyCookiesAs(capture.Cookies, 'console')}>
								<svg class="w-4 h-4 mr-2 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/></svg>
								Console (document.cookie)
							</button>
							<button class="dropdown-item" on:click={() => copyCookiesAs(capture.Cookies, 'raw')}>
								<svg class="w-4 h-4 mr-2 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4"/></svg>
								Raw JSON
							</button>
							<div class="dropdown-divider"></div>
							<button class="dropdown-item" on:click={() => sendToCookieStore(capture)}>
								<svg class="w-4 h-4 mr-2 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 4H6a2 2 0 00-2 2v12a2 2 0 002 2h12a2 2 0 002-2V6a2 2 0 00-2-2h-2m-4-1v8m0 0l3-3m-3 3L9 8m-5 5h2.586a1 1 0 01.707.293l2.414 2.414a1 1 0 00.707.293h3.172a1 1 0 00.707-.293l2.414-2.414a1 1 0 01.707-.293H20"/></svg>
								Send to Cookie Store
							</button>
						{/if}
						{#if capture.CapturedData}
							<button class="dropdown-item" on:click={() => copyToClipboard(capture.CapturedData)}>
								<svg class="w-4 h-4 mr-2 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"/></svg>
								Copy All Data
							</button>
						{/if}
						<div class="dropdown-divider"></div>
						<TableDeleteButton on:click={() => openDeleteAlert(capture)} />
					</TableDropDownEllipsis>
				</TableCellAction>
			</TableRow>

			{#if expandedRow === capture.ID}
				<tr class="expanded-row">
					<td colspan="8">
						<div class="capture-details">
							<div class="detail-grid">
								<div class="detail-item">
									<span class="detail-label">Session ID</span>
									<span class="detail-value"><code>{capture.SessionID || '-'}</code></span>
								</div>
								<div class="detail-item">
									<span class="detail-label">User Agent</span>
									<span class="detail-value" style="word-break: break-all;">{capture.UserAgent || '-'}</span>
								</div>
								<div class="detail-item">
									<span class="detail-label">Phish Domain</span>
									<span class="detail-value"><code>{capture.PhishDomain || '-'}</code></span>
								</div>
								<div class="detail-item">
									<span class="detail-label">Target Domain</span>
									<span class="detail-value"><code>{capture.TargetDomain || '-'}</code></span>
								</div>
							{#if capture.Cookies}
								<div class="detail-item full-width">
									<div class="detail-label-row">
										<span class="detail-label">Cookies ({getCookieCount(capture.Cookies)})</span>
										<div class="copy-format-btns">
											<button class="format-btn" on:click={() => copyCookiesAs(capture.Cookies, 'json')} title="Copy as JSON (Browser Extension format)">JSON</button>
											<button class="format-btn" on:click={() => copyCookiesAs(capture.Cookies, 'netscape')} title="Copy as Netscape cookies.txt">Netscape</button>
											<button class="format-btn" on:click={() => copyCookiesAs(capture.Cookies, 'header')} title="Copy as Cookie header string">Header</button>
											<button class="format-btn" on:click={() => copyCookiesAs(capture.Cookies, 'console')} title="Copy as document.cookie console commands">Console</button>
										</div>
									</div>
									{#if getCookieDomains(capture.Cookies).length > 0}
										<div class="cookie-domains">
											{#each getCookieDomains(capture.Cookies) as domain}
												<span class="domain-tag">{domain}</span>
											{/each}
										</div>
									{/if}
									<pre class="detail-pre">{JSON.stringify(toBrowserExtensionFormat(capture.Cookies), null, 2)}</pre>
								</div>
							{/if}
								{#if capture.CapturedData}
									<div class="detail-item full-width">
										<span class="detail-label">All Captured Data</span>
										<pre class="detail-pre">{(() => {
											try {
												return JSON.stringify(JSON.parse(capture.CapturedData), null, 2);
											} catch {
												return capture.CapturedData;
											}
										})()}</pre>
									</div>
								{/if}
							</div>
						</div>
					</td>
				</tr>
			{/if}
		{/each}
	</Table>

	<DeleteAlert
		list={[]}
		name={deleteValues.ip}
		onClick={() => deleteCapture(deleteValues.id)}
		bind:isVisible={isDeleteAlertVisible}
	/>
	<DeleteAlert
		list={[]}
		name={'all proxy captures'}
		onClick={() => deleteAllCaptures()}
		bind:isVisible={isDeleteAllAlertVisible}
	/>
</main>

<style>
	.controls-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1rem;
		flex-wrap: wrap;
	}
	.filter-group {
		display: flex;
		gap: 0;
		border-radius: 8px;
		overflow: hidden;
		border: 1px solid var(--border-color, #ccc);
	}
	.filter-btn {
		padding: 0.5rem 1rem;
		font-size: 0.85rem;
		font-weight: 500;
		border: none;
		background: var(--bg-secondary, #f5f5f5);
		color: var(--text-primary, #333);
		cursor: pointer;
		transition: all 0.2s ease;
		border-right: 1px solid var(--border-color, #ccc);
	}
	.filter-btn:last-child {
		border-right: none;
	}
	.filter-btn:hover {
		background: var(--bg-hover, #e0e0e0);
	}
	.filter-btn.active {
		background: var(--primary-color, #4f46e5);
		color: white;
	}
	.credential-badge {
		display: inline-flex;
		align-items: center;
		padding: 2px 8px;
		font-size: 0.8rem;
		font-weight: 600;
		border-radius: 4px;
		background: #10b98120;
		color: #059669;
		border: 1px solid #10b98140;
	}
	.small-btn {
		padding: 2px 8px;
		font-size: 0.75rem;
		border: 1px solid var(--border-color, #ccc);
		border-radius: 4px;
		background: var(--bg-secondary, #f5f5f5);
		color: var(--text-primary, #333);
		cursor: pointer;
		white-space: nowrap;
	}
	.small-btn:hover {
		background: var(--bg-hover, #e0e0e0);
	}
	.expanded-row td {
		padding: 1rem 1.5rem;
		background: var(--bg-secondary, #f9f9f9);
		border-bottom: 1px solid var(--border-color, #e0e0e0);
	}
	.capture-details {
		max-width: 100%;
	}
	.detail-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.75rem;
	}
	.detail-item {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}
	.detail-item.full-width {
		grid-column: 1 / -1;
	}
	.detail-label {
		font-weight: 600;
		font-size: 0.8rem;
		opacity: 0.7;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.detail-label-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 0.5rem;
	}
	.detail-value {
		font-size: 0.9rem;
	}
	.detail-pre {
		background: var(--bg-code, #1e1e1e);
		color: var(--text-code, #d4d4d4);
		padding: 0.75rem;
		border-radius: 6px;
		font-size: 0.8rem;
		overflow-x: auto;
		max-height: 200px;
		white-space: pre-wrap;
		word-break: break-all;
	}
	.copy-format-btns {
		display: flex;
		gap: 0.25rem;
	}
	.format-btn {
		padding: 3px 10px;
		font-size: 0.7rem;
		font-weight: 600;
		border: 1px solid var(--border-color, #ccc);
		border-radius: 4px;
		background: var(--bg-secondary, #f5f5f5);
		color: var(--text-primary, #333);
		cursor: pointer;
		text-transform: uppercase;
		letter-spacing: 0.03em;
		transition: all 0.15s ease;
	}
	.format-btn:hover {
		background: var(--primary-color, #4f46e5);
		color: white;
		border-color: var(--primary-color, #4f46e5);
	}
	.cookie-domains {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem;
		margin-bottom: 0.5rem;
	}
	.domain-tag {
		display: inline-flex;
		align-items: center;
		padding: 1px 6px;
		font-size: 0.65rem;
		font-weight: 500;
		border-radius: 3px;
		background: rgba(59, 130, 246, 0.1);
		color: rgb(59, 130, 246);
		border: 1px solid rgba(59, 130, 246, 0.2);
	}
	:global(.dark) .domain-tag {
		background: rgba(96, 165, 250, 0.1);
		color: rgb(96, 165, 250);
		border-color: rgba(96, 165, 250, 0.2);
	}
	.dropdown-item {
		display: flex;
		align-items: center;
		width: 100%;
		padding: 0.5rem 1rem;
		text-align: left;
		border: none;
		background: none;
		cursor: pointer;
		font-size: 0.875rem;
		color: var(--text-primary, #333);
	}
	.dropdown-item:hover {
		background: var(--bg-hover, #f0f0f0);
	}
	.dropdown-divider {
		height: 1px;
		margin: 0.25rem 0;
		background: var(--border-color, #e0e0e0);
	}
	.dropdown-section-label {
		padding: 0.25rem 1rem;
		font-size: 0.7rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		opacity: 0.5;
	}
	.cookie-badge {
		display: inline-flex;
		align-items: center;
		padding: 2px 8px;
		font-size: 0.7rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		border-radius: 9999px;
		background: #f59e0b20;
		color: #d97706;
		border: 1px solid #f59e0b40;
	}
</style>
