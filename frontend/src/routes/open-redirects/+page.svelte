<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { addToast } from '$lib/store/toast';
	import BigButton from '$lib/components/BigButton.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import FormGrid from '$lib/components/FormGrid.svelte';
	import FormFooter from '$lib/components/FormFooter.svelte';
	import TextField from '$lib/components/TextField.svelte';
	import TextareaField from '$lib/components/TextareaField.svelte';
	import Table from '$lib/components/table/Table.svelte';
	import TableRow from '$lib/components/table/TableRow.svelte';
	import TableCell from '$lib/components/table/TableCell.svelte';
	import TableCellAction from '$lib/components/table/TableCellAction.svelte';
	import TableCellEmpty from '$lib/components/table/TableCellEmpty.svelte';
	import TableDropDownEllipsis from '$lib/components/table/TableDropDownEllipsis.svelte';
	import TableUpdateButton from '$lib/components/table/TableUpdateButton.svelte';
	import TableDeleteButton from '$lib/components/table/TableDeleteButton2.svelte';
	import DeleteAlert from '$lib/components/modal/DeleteAlert.svelte';
	import { newTableURLParams } from '$lib/service/tableURLParams.js';
	import { AppStateService } from '$lib/service/appState';
	import FormError from '$lib/components/FormError.svelte';

	// --- State ---
	const appStateService = AppStateService.instance;
	const tableURLParams = newTableURLParams();
	let contextCompanyID = null;
	let redirects = [];
	let redirectsHasNextPage = false;
	let isTableLoading = false;

	// Active tab
	let activeTab = 'redirects'; // 'redirects' | 'sources' | 'recommendations'

	// Create/Update modal
	let isModalVisible = false;
	let modalMode = null; // 'create' | 'update'
	let isSubmitting = false;
	let formError = '';
	let form = null;
	let formValues = {
		id: null,
		name: '',
		baseURL: '',
		paramName: '',
		provider: '',
		notes: '',
		useWithProxy: false,
		proxyDomain: ''
	};

	// Delete alert
	let isDeleteAlertVisible = false;
	let deleteValues = { id: null, name: null };

	// Test modal
	let isTestModalVisible = false;
	let testResult = null;
	let isTesting = false;
	let testRedirectId = null;
	let testRedirectName = '';

	// Generate link modal
	let isGenerateLinkModalVisible = false;
	let generateLinkRedirectId = null;
	let generateLinkName = '';
	let generateTargetURL = '';
	let generatedLink = '';
	let isGenerating = false;

	// Sources modal
	let sources = [];
	let sourcesLoading = false;
	let isImportingSource = false;
	let importingSourceId = '';

	// Recommendations
	let recommendations = [];
	let recommendationsLoading = false;

	// Stats
	let stats = null;

	// Bulk test
	let selectedIds = [];
	let isBulkTesting = false;
	let bulkTestResults = null;

	// Proxy domains for integration
	let proxyDomains = [];

	onMount(() => {
		const context = appStateService.getContext();
		if (context) {
			contextCompanyID = context.companyID;
		}
		refreshRedirects();
		tableURLParams.onChange(refreshRedirects);
		loadProxyDomains();
		return () => {
			tableURLParams.unsubscribe();
		};
	});

	// --- Data Loading ---
	const refreshRedirects = async (showLoading = true) => {
		try {
			if (showLoading) isTableLoading = true;
			const res = await api.openRedirects.getAll(tableURLParams, contextCompanyID);
			if (res.success) {
				redirects = res.data.rows || [];
				redirectsHasNextPage = res.data.hasNextPage || false;
			} else {
				addToast(res.error || 'Failed to load redirects', 'Error');
			}
		} catch (e) {
			addToast('Failed to load redirects', 'Error');
		} finally {
			if (showLoading) isTableLoading = false;
		}
	};

	async function loadProxyDomains() {
		try {
			const res = await api.domain.getProxyDomains({
				currentPage: 1,
				perPage: 1000,
				sortBy: 'name',
				sortOrder: 'asc',
				search: ''
			});
			if (res && res.data && res.data.rows) {
				proxyDomains = res.data.rows;
			}
		} catch (e) {
			proxyDomains = [];
		}
	}

	async function loadSources() {
		sourcesLoading = true;
		try {
			const res = await api.openRedirects.getSources();
			if (res.success) {
				sources = res.data || [];
			}
		} catch (e) {
			addToast('Failed to load sources', 'Error');
		}
		sourcesLoading = false;
	}

	async function loadRecommendations() {
		recommendationsLoading = true;
		try {
			const res = await api.openRedirects.getRecommendations();
			if (res.success) {
				recommendations = res.data || [];
			}
		} catch (e) {
			addToast('Failed to load recommendations', 'Error');
		}
		recommendationsLoading = false;
	}

	async function loadStats() {
		try {
			const res = await api.openRedirects.getStats();
			if (res.success) {
				stats = res.data;
			}
		} catch (e) {
			console.error('Failed to load stats', e);
		}
	}

	// --- CRUD Operations ---
	function openCreateModal() {
		modalMode = 'create';
		formValues = {
			id: null,
			name: '',
			baseURL: '',
			paramName: 'url',
			provider: '',
			notes: '',
			useWithProxy: false,
			proxyDomain: ''
		};
		formError = '';
		isModalVisible = true;
	}

	async function openUpdateModal(id) {
		try {
			showIsLoading();
			const res = await api.openRedirects.getByID(id);
			if (res.success) {
				const r = res.data;
				modalMode = 'update';
				formValues = {
					id: r.id,
					name: r.name || '',
					baseURL: r.base_url || '',
					paramName: r.param_name || 'url',
					provider: r.provider || '',
					notes: r.notes || '',
					useWithProxy: r.use_with_proxy || false,
					proxyDomain: r.proxy_domain || ''
				};
				formError = '';
				isModalVisible = true;
			} else {
				addToast(res.error || 'Failed to load redirect', 'Error');
			}
		} catch (e) {
			addToast('Failed to load redirect', 'Error');
		} finally {
			hideIsLoading();
		}
	}

	async function onSubmit() {
		isSubmitting = true;
		formError = '';
		try {
			const payload = {
				name: formValues.name,
				base_url: formValues.baseURL,
				param_name: formValues.paramName,
				provider: formValues.provider,
				notes: formValues.notes,
				use_with_proxy: formValues.useWithProxy,
				proxy_domain: formValues.proxyDomain
			};

			let res;
			if (modalMode === 'create') {
				res = await api.openRedirects.create(payload);
			} else {
				res = await api.openRedirects.update(formValues.id, payload);
			}

			if (res.success) {
				addToast(
					modalMode === 'create' ? 'Redirect created' : 'Redirect updated',
					'Success'
				);
				closeModal();
				await refreshRedirects();
			} else {
				formError = res.error || 'Failed to save redirect';
			}
		} catch (e) {
			formError = 'Failed to save redirect';
		}
		isSubmitting = false;
	}

	function closeModal() {
		isModalVisible = false;
		modalMode = null;
		formError = '';
	}

	function openDeleteAlert(redirect) {
		deleteValues = { id: redirect.id, name: redirect.name };
		isDeleteAlertVisible = true;
	}

	async function confirmDelete() {
		try {
			const res = await api.openRedirects.deleteByID(deleteValues.id);
			if (res.success) {
				addToast('Redirect deleted', 'Success');
				await refreshRedirects();
			} else {
				addToast(res.error || 'Failed to delete', 'Error');
			}
		} catch (e) {
			addToast('Failed to delete redirect', 'Error');
		}
		isDeleteAlertVisible = false;
	}

	// --- Test Operations ---
	async function testRedirect(id, name) {
		testRedirectId = id;
		testRedirectName = name;
		testResult = null;
		isTesting = true;
		isTestModalVisible = true;

		try {
			const res = await api.openRedirects.test(id);
			if (res.success) {
				testResult = res.data;
			} else {
				testResult = { working: false, error: res.error || 'Test failed' };
			}
		} catch (e) {
			testResult = { working: false, error: 'Test request failed' };
		}
		isTesting = false;
	}

	async function bulkTest() {
		if (selectedIds.length === 0) {
			addToast('Select redirects to test', 'Error');
			return;
		}
		isBulkTesting = true;
		bulkTestResults = null;
		try {
			const res = await api.openRedirects.bulkTest(selectedIds);
			if (res.success) {
				bulkTestResults = res.data;
				addToast(`Tested ${res.data.length} redirects`, 'Success');
				await refreshRedirects();
			} else {
				addToast(res.error || 'Bulk test failed', 'Error');
			}
		} catch (e) {
			addToast('Bulk test failed', 'Error');
		}
		isBulkTesting = false;
	}

	function toggleSelectAll() {
		if (selectedIds.length === redirects.length) {
			selectedIds = [];
		} else {
			selectedIds = redirects.map((r) => r.id);
		}
	}

	function toggleSelect(id) {
		if (selectedIds.includes(id)) {
			selectedIds = selectedIds.filter((i) => i !== id);
		} else {
			selectedIds = [...selectedIds, id];
		}
	}

	// --- Generate Link ---
	function openGenerateLinkModal(redirect) {
		generateLinkRedirectId = redirect.id;
		generateLinkName = redirect.name;
		generateTargetURL = '';
		generatedLink = '';
		isGenerating = false;
		isGenerateLinkModalVisible = true;
	}

	async function generateLink() {
		if (!generateTargetURL) {
			addToast('Enter a target URL', 'Error');
			return;
		}
		isGenerating = true;
		try {
			const res = await api.openRedirects.generateLink(generateLinkRedirectId, generateTargetURL);
			if (res.success) {
				generatedLink = res.data.generated_url || res.data.url || '';
			} else {
				addToast(res.error || 'Failed to generate link', 'Error');
			}
		} catch (e) {
			addToast('Failed to generate link', 'Error');
		}
		isGenerating = false;
	}

	function copyToClipboard(text) {
		navigator.clipboard.writeText(text).then(() => {
			addToast('Copied to clipboard', 'Success');
		});
	}

	// --- Import from Source ---
	async function importFromSource(sourceId) {
		importingSourceId = sourceId;
		isImportingSource = true;
		try {
			const res = await api.openRedirects.importFromSource(sourceId);
			if (res.success) {
				const count = res.data?.imported || 0;
				addToast(`Imported ${count} redirect(s)`, 'Success');
				await refreshRedirects();
			} else {
				addToast(res.error || 'Import failed', 'Error');
			}
		} catch (e) {
			addToast('Import failed', 'Error');
		}
		isImportingSource = false;
		importingSourceId = '';
	}

	// --- Toggle active ---
	async function toggleActive(id) {
		try {
			const res = await api.openRedirects.toggle(id);
			if (res.success) {
				await refreshRedirects(false);
			} else {
				addToast(res.error || 'Toggle failed', 'Error');
			}
		} catch (e) {
			addToast('Toggle failed', 'Error');
		}
	}

	// --- Tab switching ---
	function switchTab(tab) {
		activeTab = tab;
		if (tab === 'sources' && sources.length === 0) {
			loadSources();
		}
		if (tab === 'recommendations' && recommendations.length === 0) {
			loadRecommendations();
		}
	}

	// Status badge helper
	function getStatusColor(status) {
		switch (status) {
			case 'working':
				return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
			case 'failed':
				return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200';
			case 'untested':
				return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200';
			default:
				return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200';
		}
	}

	function formatDate(dateStr) {
		if (!dateStr) return '-';
		try {
			return new Date(dateStr).toLocaleString();
		} catch {
			return dateStr;
		}
	}
</script>

<HeadTitle value="Open Redirects" />

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<Headline>Open Redirects</Headline>
		<div class="flex gap-2">
			{#if activeTab === 'redirects' && selectedIds.length > 0}
				<button
					class="px-4 py-2 text-sm font-medium rounded-lg border border-amber-500 text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-900/20 transition-colors"
					on:click={bulkTest}
					disabled={isBulkTesting}
				>
					{isBulkTesting ? 'Testing...' : `Test Selected (${selectedIds.length})`}
				</button>
			{/if}
			{#if activeTab === 'redirects'}
				<BigButton on:click={openCreateModal}>Add Redirect</BigButton>
			{/if}
		</div>
	</div>

	<!-- Tab Navigation -->
	<div class="flex border-b border-gray-200 dark:border-gray-700">
		<button
			class="px-4 py-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'redirects'
				? 'border-highlight-blue text-highlight-blue'
				: 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}"
			on:click={() => switchTab('redirects')}
		>
			Redirects
		</button>
		<button
			class="px-4 py-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'sources'
				? 'border-highlight-blue text-highlight-blue'
				: 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}"
			on:click={() => switchTab('sources')}
		>
			Known Sources
		</button>
		<button
			class="px-4 py-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'recommendations'
				? 'border-highlight-blue text-highlight-blue'
				: 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}"
			on:click={() => switchTab('recommendations')}
		>
			Tools & Resources
		</button>
	</div>

	<!-- Redirects Tab -->
	{#if activeTab === 'redirects'}
		<Table
			columns={[
				{ column: '', size: 'tiny' },
				{ column: 'Name', size: 'medium' },
				{ column: 'Provider', size: 'small' },
				{ column: 'Base URL', size: 'large' },
				{ column: 'Status', size: 'small' },
				{ column: 'Last Tested', size: 'medium' }
			]}
			sortable={['Name', 'Provider', 'Status', 'Last Tested']}
			hasData={!!redirects.length}
			hasNextPage={redirectsHasNextPage}
			plural="Redirects"
			pagination={tableURLParams}
			isGhost={isTableLoading}
		>
			{#each redirects as redirect}
				<TableRow>
					<TableCell>
						<input
							type="checkbox"
							checked={selectedIds.includes(redirect.id)}
							on:change={() => toggleSelect(redirect.id)}
							class="rounded border-gray-300 dark:border-gray-600 text-highlight-blue focus:ring-highlight-blue"
						/>
					</TableCell>
					<TableCell>
						<button
							on:click={() => openUpdateModal(redirect.id)}
							class="block w-full py-1 text-left font-medium text-slate-700 dark:text-gray-200 hover:text-highlight-blue transition-colors"
							title={redirect.name}
						>
							{redirect.name}
						</button>
					</TableCell>
					<TableCell>
						{#if redirect.provider}
							<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
								{redirect.provider}
							</span>
						{:else}
							<span class="text-gray-400">-</span>
						{/if}
					</TableCell>
					<TableCell>
						<span class="text-xs font-mono text-gray-600 dark:text-gray-400 truncate block max-w-xs" title={redirect.base_url}>
							{redirect.base_url}
						</span>
					</TableCell>
					<TableCell>
						<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {getStatusColor(redirect.status)}">
							{redirect.status || 'untested'}
						</span>
					</TableCell>
					<TableCell>
						<span class="text-xs text-gray-500 dark:text-gray-400">
							{formatDate(redirect.last_tested_at)}
						</span>
					</TableCell>
					<TableCellEmpty />
					<TableCellAction>
						<TableDropDownEllipsis>
							<TableUpdateButton on:click={() => openUpdateModal(redirect.id)} />
							<button
								class="w-full px py-1 text-slate-600 dark:text-gray-200 hover:bg-highlight-blue dark:hover:bg-highlight-blue/50 hover:text-white cursor-pointer text-left transition-colors duration-200"
								on:click={() => testRedirect(redirect.id, redirect.name)}
								title="Test Redirect"
							>
								<p class="ml-2 text-left">Test Redirect</p>
							</button>
							<button
								class="w-full px py-1 text-slate-600 dark:text-gray-200 hover:bg-highlight-blue dark:hover:bg-highlight-blue/50 hover:text-white cursor-pointer text-left transition-colors duration-200"
								on:click={() => openGenerateLinkModal(redirect)}
								title="Generate Link"
							>
								<p class="ml-2 text-left">Generate Link</p>
							</button>
							<button
								class="w-full px py-1 text-slate-600 dark:text-gray-200 hover:bg-highlight-blue dark:hover:bg-highlight-blue/50 hover:text-white cursor-pointer text-left transition-colors duration-200"
								on:click={() => toggleActive(redirect.id)}
								title={redirect.is_active ? 'Disable' : 'Enable'}
							>
								<p class="ml-2 text-left">{redirect.is_active ? 'Disable' : 'Enable'}</p>
							</button>
							<TableDeleteButton on:click={() => openDeleteAlert(redirect)} />
						</TableDropDownEllipsis>
					</TableCellAction>
				</TableRow>
			{/each}
		</Table>
	{/if}

	<!-- Known Sources Tab -->
	{#if activeTab === 'sources'}
		<div class="space-y-4">
			<p class="text-sm text-gray-600 dark:text-gray-400">
				Known open redirect endpoints from major providers. Import them with one click, then test to verify they're still active.
			</p>
			{#if sourcesLoading}
				<div class="flex justify-center py-8">
					<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-highlight-blue"></div>
				</div>
			{:else}
				<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
					{#each sources as source}
						<div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 hover:border-highlight-blue/50 transition-colors">
							<div class="flex items-start justify-between">
								<div class="flex-1 min-w-0">
									<h3 class="text-sm font-semibold text-gray-900 dark:text-white truncate">
										{source.name || source.provider}
									</h3>
									<p class="text-xs text-gray-500 dark:text-gray-400 mt-1 truncate" title={source.base_url}>
										{source.base_url}
									</p>
									{#if source.description}
										<p class="text-xs text-gray-400 dark:text-gray-500 mt-1">
											{source.description}
										</p>
									{/if}
								</div>
								<button
									class="ml-2 flex-shrink-0 px-3 py-1.5 text-xs font-medium rounded-md bg-highlight-blue text-white hover:bg-highlight-blue/80 transition-colors disabled:opacity-50"
									on:click={() => importFromSource(source.id)}
									disabled={isImportingSource && importingSourceId === source.id}
								>
									{isImportingSource && importingSourceId === source.id ? 'Importing...' : 'Import'}
								</button>
							</div>
							<div class="mt-2 flex items-center gap-2">
								<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
									{source.provider}
								</span>
								<span class="text-xs text-gray-400">
									Param: <code class="bg-gray-100 dark:bg-gray-800 px-1 rounded">{source.param_name}</code>
								</span>
							</div>
						</div>
					{/each}
				</div>
				{#if sources.length === 0}
					<div class="text-center py-8 text-gray-500 dark:text-gray-400">
						No known sources available.
					</div>
				{/if}
			{/if}
		</div>
	{/if}

	<!-- Tools & Resources Tab -->
	{#if activeTab === 'recommendations'}
		<div class="space-y-6">
			<p class="text-sm text-gray-600 dark:text-gray-400">
				Open-source tools for discovering and testing open redirect vulnerabilities. Use these to find new redirect endpoints.
			</p>
			{#if recommendationsLoading}
				<div class="flex justify-center py-8">
					<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-highlight-blue"></div>
				</div>
			{:else}
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					{#each recommendations as tool}
						<div class="border border-gray-200 dark:border-gray-700 rounded-lg p-5 hover:shadow-md transition-shadow">
							<div class="flex items-start gap-3">
								<div class="flex-shrink-0 w-10 h-10 rounded-lg bg-gradient-to-br from-highlight-blue/20 to-highlight-blue/5 flex items-center justify-center">
									<svg class="w-5 h-5 text-highlight-blue" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
									</svg>
								</div>
								<div class="flex-1 min-w-0">
									<h3 class="text-sm font-semibold text-gray-900 dark:text-white">
										{tool.name}
									</h3>
									<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
										{tool.description}
									</p>
									{#if tool.install_command}
										<div class="mt-2 bg-gray-900 dark:bg-gray-800 rounded-md p-2">
											<code class="text-xs text-green-400 font-mono">{tool.install_command}</code>
										</div>
									{/if}
									{#if tool.url}
										<a
											href={tool.url}
											target="_blank"
											rel="noopener noreferrer"
											class="inline-flex items-center gap-1 mt-2 text-xs text-highlight-blue hover:underline"
										>
											View on GitHub
											<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
											</svg>
										</a>
									{/if}
								</div>
							</div>
						</div>
					{/each}
				</div>
				{#if recommendations.length === 0}
					<div class="text-center py-8 text-gray-500 dark:text-gray-400">
						No recommendations available.
					</div>
				{/if}
			{/if}
		</div>
	{/if}
</div>

<!-- Create/Update Modal -->
<Modal
	headerText={modalMode === 'create' ? 'Add Open Redirect' : 'Update Open Redirect'}
	visible={isModalVisible}
	onClose={closeModal}
	{isSubmitting}
>
	<FormGrid on:submit={onSubmit} bind:bindTo={form} {isSubmitting} {modalMode}>
		<div class="col-span-3 w-full px-6 py-4 overflow-y-auto space-y-4">
			{#if formError}
				<FormError error={formError} />
			{/if}

			<TextField
				label="Name"
				placeholder="e.g., Google AMP Redirect"
				bind:value={formValues.name}
				required
			/>

			<TextField
				label="Base URL"
				placeholder="e.g., https://www.google.com/url"
				bind:value={formValues.baseURL}
				required
			/>

			<TextField
				label="Parameter Name"
				placeholder="e.g., url, q, redirect_uri"
				bind:value={formValues.paramName}
				required
			/>

			<TextField
				label="Provider"
				placeholder="e.g., Google, Microsoft, LinkedIn"
				bind:value={formValues.provider}
			/>

			<TextareaField
				label="Notes"
				placeholder="Any notes about this redirect..."
				bind:value={formValues.notes}
				rows={3}
			/>

			<div class="border-t border-gray-200 dark:border-gray-700 pt-4">
				<h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Proxy Integration</h4>

				<label class="flex items-center gap-2 cursor-pointer">
					<input
						type="checkbox"
						bind:checked={formValues.useWithProxy}
						class="rounded border-gray-300 dark:border-gray-600 text-highlight-blue focus:ring-highlight-blue"
					/>
					<span class="text-sm text-gray-700 dark:text-gray-300">Use with proxy domain</span>
				</label>

				{#if formValues.useWithProxy}
					<div class="mt-3">
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Proxy Domain
						</label>
						<select
							bind:value={formValues.proxyDomain}
							class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 px-3 py-2 text-sm focus:ring-highlight-blue focus:border-highlight-blue"
						>
							<option value="">Select a proxy domain...</option>
							{#each proxyDomains as domain}
								<option value={domain.name || domain.domain}>{domain.name || domain.domain}</option>
							{/each}
						</select>
					</div>
				{/if}
			</div>
		</div>
		<FormFooter slot="footer" {isSubmitting} {modalMode} onCancel={closeModal} />
	</FormGrid>
</Modal>

<!-- Delete Alert -->
<DeleteAlert
	visible={isDeleteAlertVisible}
	name={deleteValues.name}
	onClose={() => (isDeleteAlertVisible = false)}
	onDelete={confirmDelete}
/>

<!-- Test Result Modal -->
<Modal
	headerText="Test Result: {testRedirectName}"
	visible={isTestModalVisible}
	onClose={() => (isTestModalVisible = false)}
>
	<div class="p-6 space-y-4">
		{#if isTesting}
			<div class="flex items-center justify-center py-8">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-highlight-blue"></div>
				<span class="ml-3 text-gray-600 dark:text-gray-400">Testing redirect...</span>
			</div>
		{:else if testResult}
			<div class="rounded-lg p-4 {testResult.working ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800' : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'}">
				<div class="flex items-center gap-2">
					{#if testResult.working}
						<svg class="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
						</svg>
						<span class="font-medium text-green-800 dark:text-green-200">Redirect is working</span>
					{:else}
						<svg class="w-5 h-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
						<span class="font-medium text-red-800 dark:text-red-200">Redirect failed</span>
					{/if}
				</div>
			</div>

			{#if testResult.status_code}
				<div class="text-sm">
					<span class="text-gray-500 dark:text-gray-400">Status Code:</span>
					<span class="font-mono font-medium ml-1">{testResult.status_code}</span>
				</div>
			{/if}

			{#if testResult.final_url}
				<div class="text-sm">
					<span class="text-gray-500 dark:text-gray-400">Final URL:</span>
					<code class="block mt-1 p-2 bg-gray-100 dark:bg-gray-800 rounded text-xs font-mono break-all">
						{testResult.final_url}
					</code>
				</div>
			{/if}

			{#if testResult.redirect_chain && testResult.redirect_chain.length > 0}
				<div class="text-sm">
					<span class="text-gray-500 dark:text-gray-400">Redirect Chain ({testResult.redirect_chain.length} hops):</span>
					<div class="mt-1 space-y-1">
						{#each testResult.redirect_chain as hop, i}
							<div class="flex items-center gap-2 text-xs">
								<span class="flex-shrink-0 w-5 h-5 rounded-full bg-gray-200 dark:bg-gray-700 flex items-center justify-center text-gray-600 dark:text-gray-400 font-medium">
									{i + 1}
								</span>
								<code class="font-mono text-gray-600 dark:text-gray-400 truncate">{hop}</code>
							</div>
						{/each}
					</div>
				</div>
			{/if}

			{#if testResult.error}
				<div class="text-sm text-red-600 dark:text-red-400">
					{testResult.error}
				</div>
			{/if}
		{/if}
	</div>
</Modal>

<!-- Generate Link Modal -->
<Modal
	headerText="Generate Redirect Link: {generateLinkName}"
	visible={isGenerateLinkModalVisible}
	onClose={() => (isGenerateLinkModalVisible = false)}
>
	<div class="p-6 space-y-4">
		<TextField
			label="Target URL (your phishing page or proxy domain)"
			placeholder="https://your-proxy-domain.com/login"
			bind:value={generateTargetURL}
		/>

		<button
			class="w-full px-4 py-2 text-sm font-medium rounded-lg bg-highlight-blue text-white hover:bg-highlight-blue/80 transition-colors disabled:opacity-50"
			on:click={generateLink}
			disabled={isGenerating || !generateTargetURL}
		>
			{isGenerating ? 'Generating...' : 'Generate Link'}
		</button>

		{#if generatedLink}
			<div class="space-y-2">
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300">Generated Link</label>
				<div class="flex gap-2">
					<code class="flex-1 p-3 bg-gray-100 dark:bg-gray-800 rounded-lg text-xs font-mono break-all text-gray-800 dark:text-gray-200">
						{generatedLink}
					</code>
					<button
						class="flex-shrink-0 px-3 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
						on:click={() => copyToClipboard(generatedLink)}
						title="Copy to clipboard"
					>
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
						</svg>
					</button>
				</div>
				<p class="text-xs text-gray-500 dark:text-gray-400">
					This link will redirect through a trusted domain before reaching your target URL.
				</p>
			</div>
		{/if}
	</div>
</Modal>
