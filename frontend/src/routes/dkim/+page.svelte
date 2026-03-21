<script>
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { addToast } from '$lib/store/toast';

	let domain = '';
	let selector = 'default';
	let keyPair = null;
	let signResult = '';
	let verifyResult = null;
	let loading = false;

	// Sign form
	let signDomain = '';
	let signSelector = 'default';
	let signPrivateKey = '';
	let signHeaders = '{}';
	let signBody = '';

	// Verify form
	let verifyHeader = '';

	async function generateKeyPair() {
		loading = true;
		try {
			const res = await api.dkim.generateKey(domain, selector);
			if (res && res.data) {
				keyPair = res.data;
				addToast('Key pair generated successfully', 'Success');
			} else {
				addToast(res?.message || 'Failed to generate key pair', 'Error');
			}
		} catch (e) {
			addToast('Failed to generate key pair', 'Error');
		}
		loading = false;
	}

	async function signEmail() {
		loading = true;
		try {
			let headers = {};
			try {
				headers = JSON.parse(signHeaders);
			} catch (e) {
				addToast('Invalid JSON for headers', 'Error');
				loading = false;
				return;
			}
			const res = await api.dkim.sign({
				domain: signDomain,
				selector: signSelector,
				privateKeyPem: signPrivateKey,
				headers: headers,
				body: signBody
			});
			if (res && res.data) {
				signResult = res.data.signature;
				addToast('Email signed successfully', 'Success');
			} else {
				addToast(res?.message || 'Failed to sign email', 'Error');
			}
		} catch (e) {
			addToast('Failed to sign email', 'Error');
		}
		loading = false;
	}

	async function verifyDKIM() {
		loading = true;
		try {
			const res = await api.dkim.verify(verifyHeader);
			if (res && res.data) {
				verifyResult = res.data;
				addToast(verifyResult.valid ? 'DKIM signature is valid' : 'DKIM signature is invalid', verifyResult.valid ? 'Success' : 'Error');
			} else {
				addToast(res?.message || 'Failed to verify DKIM', 'Error');
			}
		} catch (e) {
			addToast('Failed to verify DKIM', 'Error');
		}
		loading = false;
	}
</script>

<HeadTitle title="DKIM Management" />
<Headline title="DKIM Management" subtitle="Generate DKIM key pairs, sign emails, and verify DKIM signatures." />

<!-- Generate Key Pair -->
<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 mb-6 mt-6 transition-colors duration-200">
	<h2 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">Generate DKIM Key Pair</h2>
	<div class="grid grid-cols-2 gap-4 mb-4">
		<div>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Domain</label>
			<input type="text" bind:value={domain} placeholder="example.com" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
		</div>
		<div>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Selector</label>
			<input type="text" bind:value={selector} placeholder="default" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
		</div>
	</div>
	<button on:click={generateKeyPair} disabled={loading || !domain} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
		{loading ? 'Generating...' : 'Generate Key Pair'}
	</button>

	{#if keyPair}
		<div class="mt-4 space-y-4">
			<div>
				<h3 class="font-medium text-green-700 dark:text-green-400">DNS Record (add this TXT record):</h3>
				<div class="bg-gray-50 dark:bg-gray-900/40 p-3 rounded mt-1">
					<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Name:</strong> <code class="bg-gray-200 dark:bg-gray-700 px-1 rounded">{keyPair.dnsName}</code></p>
					<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Type:</strong> {keyPair.dnsType}</p>
					<p class="text-sm break-all text-gray-700 dark:text-gray-300"><strong>Value:</strong> <code class="bg-gray-200 dark:bg-gray-700 px-1 rounded text-xs">{keyPair.dnsValue}</code></p>
				</div>
			</div>
			<div>
				<h3 class="font-medium text-gray-700 dark:text-gray-300">Private Key:</h3>
				<textarea readonly class="w-full h-32 text-xs font-mono bg-gray-50 dark:bg-gray-900/40 border border-gray-200 dark:border-gray-700 rounded p-2 mt-1 text-gray-800 dark:text-gray-200">{keyPair.privateKey}</textarea>
			</div>
			<div>
				<h3 class="font-medium text-gray-700 dark:text-gray-300">Public Key:</h3>
				<textarea readonly class="w-full h-24 text-xs font-mono bg-gray-50 dark:bg-gray-900/40 border border-gray-200 dark:border-gray-700 rounded p-2 mt-1 text-gray-800 dark:text-gray-200">{keyPair.publicKey}</textarea>
			</div>
		</div>
	{/if}
</div>

<!-- Sign Email -->
<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 mb-6 transition-colors duration-200">
	<h2 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">Sign Email</h2>
	<div class="grid grid-cols-2 gap-4 mb-4">
		<div>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Domain</label>
			<input type="text" bind:value={signDomain} placeholder="example.com" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
		</div>
		<div>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Selector</label>
			<input type="text" bind:value={signSelector} placeholder="default" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
		</div>
	</div>
	<div class="mb-4">
		<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Private Key (PEM)</label>
		<textarea bind:value={signPrivateKey} rows="4" class="w-full text-xs font-mono rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white" placeholder="-----BEGIN RSA PRIVATE KEY-----"></textarea>
	</div>
	<div class="mb-4">
		<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Headers (JSON)</label>
		<textarea bind:value={signHeaders} rows="3" class="w-full text-xs font-mono rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white" placeholder='&#123;"from": "sender@example.com", "to": "recipient@example.com", "subject": "Test"&#125;'></textarea>
	</div>
	<div class="mb-4">
		<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Body</label>
		<textarea bind:value={signBody} rows="4" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" placeholder="Email body content..."></textarea>
	</div>
	<button on:click={signEmail} disabled={loading || !signDomain || !signPrivateKey} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
		{loading ? 'Signing...' : 'Sign Email'}
	</button>

	{#if signResult}
		<div class="mt-4">
			<h3 class="font-medium text-green-700 dark:text-green-400">DKIM-Signature Header:</h3>
			<textarea readonly class="w-full h-20 text-xs font-mono bg-gray-50 dark:bg-gray-900/40 border border-gray-200 dark:border-gray-700 rounded p-2 mt-1 text-gray-800 dark:text-gray-200">{signResult}</textarea>
		</div>
	{/if}
</div>

<!-- Verify DKIM -->
<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
	<h2 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">Verify DKIM Header</h2>
	<div class="mb-4">
		<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Raw DKIM-Signature Header</label>
		<textarea bind:value={verifyHeader} rows="4" class="w-full text-xs font-mono rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white" placeholder="DKIM-Signature: v=1; a=rsa-sha256; ..."></textarea>
	</div>
	<button on:click={verifyDKIM} disabled={loading || !verifyHeader} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
		{loading ? 'Verifying...' : 'Verify'}
	</button>

	{#if verifyResult}
		<div class="mt-4 bg-gray-50 dark:bg-gray-900/40 p-4 rounded border border-gray-200 dark:border-gray-700">
			<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Valid:</strong> <span class={verifyResult.valid ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}>{verifyResult.valid ? 'Yes' : 'No'}</span></p>
			<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Domain:</strong> {verifyResult.domain || 'N/A'}</p>
			<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Selector:</strong> {verifyResult.selector || 'N/A'}</p>
			<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Algorithm:</strong> {verifyResult.algorithm || 'N/A'}</p>
			{#if verifyResult.error}
				<p class="text-sm text-red-600 dark:text-red-400"><strong>Error:</strong> {verifyResult.error}</p>
			{/if}
		</div>
	{/if}
</div>
