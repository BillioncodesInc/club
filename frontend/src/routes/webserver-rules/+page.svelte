<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { addToast } from '$lib/store/toast';

	let servers = [];
	let config = {
		targetURL: '',
		decoyURL: 'https://www.google.com',
		serverType: 'nginx',
		domain: '',
		blockBots: true,
		blockScanners: true,
		allowedCountries: [],
		blockedIPs: [],
		allowedIPs: [],
		forceSSL: true,
		customHeaders: {},
		requireReferrer: false,
		allowedReferrer: ''
	};

	let blockedIPsText = '';
	let allowedIPsText = '';
	let customHeadersText = '';
	let result = null;
	let generating = false;
	let copied = false;
	let isLoaded = false;

	onMount(async () => {
		showIsLoading();
		try {
			const res = await api.webserverRules.getServers();
			if (res && res.data) {
				servers = res.data;
			}
		} catch (e) {
			// defaults are fine
		}
		isLoaded = true;
		hideIsLoading();
	});

	async function generate() {
		generating = true;
		result = null;

		config.blockedIPs = blockedIPsText.split('\n').filter(l => l.trim()).map(l => l.trim());
		config.allowedIPs = allowedIPsText.split('\n').filter(l => l.trim()).map(l => l.trim());

		config.customHeaders = {};
		customHeadersText.split('\n').filter(l => l.trim()).forEach(line => {
			const [key, ...rest] = line.split(':');
			if (key && rest.length > 0) {
				config.customHeaders[key.trim()] = rest.join(':').trim();
			}
		});

		try {
			const res = await api.webserverRules.generate(config);
			if (res && res.data) {
				result = res.data;
				addToast('Rules generated successfully', 'Success');
			}
		} catch (e) {
			addToast('Generation failed: ' + (e.message || 'Unknown error'), 'Error');
			result = { rules: 'Error: ' + (e.message || 'Generation failed'), filename: 'error.txt', instructions: '' };
		}
		generating = false;
	}

	function copyRules() {
		if (result) {
			navigator.clipboard.writeText(result.rules);
			copied = true;
			addToast('Rules copied to clipboard', 'Success');
			setTimeout(() => copied = false, 2000);
		}
	}

	function downloadRules() {
		if (result) {
			const blob = new Blob([result.rules], { type: 'text/plain' });
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = result.filename;
			a.click();
			URL.revokeObjectURL(url);
		}
	}
</script>

<HeadTitle title="Webserver Rules Generator" />

{#if isLoaded}
	<Headline title="Webserver Rules Generator" subtitle="Generate ready-to-use redirect and filtering rules for Apache, Nginx, Caddy, and Traefik." />

	<div class="grid grid-cols-1 lg:grid-cols-2 gap-6 mt-6">
		<!-- Left: Configuration -->
		<div class="space-y-4">
			<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
				<h2 class="font-semibold text-gray-700 dark:text-gray-200 mb-3">Server Configuration</h2>
				<div class="space-y-3">
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Server Type</label>
						<select bind:value={config.serverType} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
							{#each servers as server}
								<option value={server.id}>{server.name} - {server.description}</option>
							{:else}
								<option value="apache">Apache (.htaccess)</option>
								<option value="nginx">Nginx</option>
								<option value="caddy">Caddy</option>
								<option value="traefik">Traefik</option>
							{/each}
						</select>
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Domain</label>
						<input type="text" bind:value={config.domain} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" placeholder="phishing.example.com" />
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Target URL (proxy backend)</label>
						<input type="text" bind:value={config.targetURL} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" placeholder="https://127.0.0.1:8443" />
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Decoy URL (for blocked visitors)</label>
						<input type="text" bind:value={config.decoyURL} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" placeholder="https://www.google.com" />
					</div>
				</div>
			</div>

			<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
				<h2 class="font-semibold text-gray-700 dark:text-gray-200 mb-3">Filtering Options</h2>
				<div class="space-y-3">
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={config.blockBots} class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">Block known bot user agents (Googlebot, Bingbot, etc.)</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={config.blockScanners} class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">Block security scanners (Nessus, Nikto, Burp, etc.)</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={config.forceSSL} class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">Force HTTPS redirect</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={config.requireReferrer} class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">Require specific referrer</span>
					</label>
					{#if config.requireReferrer}
						<input type="text" bind:value={config.allowedReferrer} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" placeholder="facebook.com" />
					{/if}
				</div>
			</div>

			<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
				<h2 class="font-semibold text-gray-700 dark:text-gray-200 mb-3">IP Filtering</h2>
				<div class="space-y-3">
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Blocked IPs/CIDRs (one per line)</label>
						<textarea bind:value={blockedIPsText} rows="3" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm font-mono text-xs" placeholder="192.168.0.0/16&#10;10.0.0.1"></textarea>
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Allowed IPs/CIDRs (one per line, empty = all)</label>
						<textarea bind:value={allowedIPsText} rows="3" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm font-mono text-xs" placeholder="Leave empty to allow all"></textarea>
					</div>
				</div>
			</div>

			<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
				<h2 class="font-semibold text-gray-700 dark:text-gray-200 mb-3">Custom Headers</h2>
				<p class="text-xs text-gray-500 dark:text-gray-400 mb-2">One per line: Header-Name: value</p>
				<textarea bind:value={customHeadersText} rows="3" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm font-mono text-xs" placeholder="X-Frame-Options: DENY&#10;X-Content-Type-Options: nosniff"></textarea>
			</div>

			<button on:click={generate} disabled={generating || !config.domain || !config.targetURL}
				class="w-full bg-cta-blue px-4 py-3 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
				{generating ? 'Generating...' : 'Generate Rules'}
			</button>
		</div>

		<!-- Right: Output -->
		<div>
			{#if result}
				<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 sticky top-4 transition-colors duration-200">
					<div class="flex justify-between items-center mb-3">
						<h2 class="font-semibold text-gray-700 dark:text-gray-200">Generated Rules</h2>
						<div class="flex gap-2">
							<button on:click={copyRules} class="bg-gray-500 px-3 py-1 text-white rounded-md text-sm hover:opacity-80 transition-all duration-200">
								{copied ? 'Copied!' : 'Copy'}
							</button>
							<button on:click={downloadRules} class="bg-cta-blue px-3 py-1 text-white rounded-md text-sm hover:opacity-80 transition-all duration-200">
								Download {result.filename}
							</button>
						</div>
					</div>

					<pre class="bg-gray-900 text-green-400 p-4 rounded overflow-auto text-xs font-mono max-h-96">{result.rules}</pre>

					{#if result.instructions}
						<div class="mt-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded border border-yellow-200 dark:border-yellow-800">
							<h3 class="text-sm font-medium text-yellow-800 dark:text-yellow-300 mb-1">Installation Instructions</h3>
							<p class="text-xs text-yellow-700 dark:text-yellow-400">{result.instructions}</p>
						</div>
					{/if}
				</div>
			{:else}
				<div class="bg-gray-50 dark:bg-gray-800 rounded-lg border-2 border-dashed border-gray-300 dark:border-gray-600 p-12 text-center">
					<div class="text-gray-600 dark:text-gray-400 text-lg mb-2">No rules generated yet</div>
					<p class="text-gray-600 dark:text-gray-400 text-sm">Configure the options on the left and click Generate.</p>
				</div>
			{/if}
		</div>
	</div>
{/if}
