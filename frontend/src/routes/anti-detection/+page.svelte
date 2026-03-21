<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { addToast } from '$lib/store/toast';

	let activeTab = 'scan';
	let loading = false;

	// Dirty Word Scanner
	let scanContent = '';
	let scanResults = null;

	// HTML Mutator
	let htmlInput = '';
	let htmlOutput = '';
	let mutationMethod = 0;
	let mutationIntensity = 0.5;

	// Text Encoder
	let textInput = '';
	let textOutput = '';
	let encodingMethod = 0;

	const mutationMethods = [
		{ value: 0, label: 'Invisible Characters' },
		{ value: 1, label: 'Zero-Width Joiners' },
		{ value: 2, label: 'HTML Entity Encoding' },
		{ value: 3, label: 'CSS Class Randomization' },
		{ value: 4, label: 'Attribute Shuffling' }
	];

	const encodingMethods = [
		{ value: 0, label: 'Homoglyph Substitution' },
		{ value: 1, label: 'Unicode Confusables' },
		{ value: 2, label: 'Mixed Encoding' }
	];

	async function scanDirtyWords() {
		loading = true;
		try {
			const res = await api.antiDetection.scanDirtyWords(scanContent);
			if (res.success) {
				scanResults = res.data;
			} else {
				addToast(res.error || 'Scan failed', 'Error');
			}
		} catch (e) {
			addToast('Scan failed', 'Error');
		}
		loading = false;
	}

	async function mutateHTML() {
		loading = true;
		try {
			const res = await api.antiDetection.mutateHTML(htmlInput, mutationMethod, mutationIntensity);
			if (res.success && res.data) {
				htmlOutput = res.data.html;
			} else {
				addToast(res.error || 'Mutation failed', 'Error');
			}
		} catch (e) {
			addToast('Mutation failed', 'Error');
		}
		loading = false;
	}

	async function encodeText() {
		loading = true;
		try {
			const res = await api.antiDetection.encodeText(textInput, encodingMethod);
			if (res.success && res.data) {
				textOutput = res.data.text;
			} else {
				addToast(res.error || 'Encoding failed', 'Error');
			}
		} catch (e) {
			addToast('Encoding failed', 'Error');
		}
		loading = false;
	}

	function copyToClipboard(text) {
		navigator.clipboard.writeText(text);
		addToast('Copied to clipboard', 'Success');
	}
</script>

<HeadTitle title="Anti-Detection Suite" />
<Headline title="Anti-Detection Suite" subtitle="Tools to evade email security scanners and spam filters." />

<!-- Tab Navigation -->
<div class="border-b border-gray-200 dark:border-gray-700 mb-6 mt-6">
	<nav class="-mb-px flex space-x-8">
		<button
			class="py-2 px-1 border-b-2 font-medium text-sm transition-colors duration-200 {activeTab === 'scan' ? 'border-cta-blue text-cta-blue' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300'}"
			on:click={() => (activeTab = 'scan')}
		>
			Dirty Word Scanner
		</button>
		<button
			class="py-2 px-1 border-b-2 font-medium text-sm transition-colors duration-200 {activeTab === 'mutate' ? 'border-cta-blue text-cta-blue' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300'}"
			on:click={() => (activeTab = 'mutate')}
		>
			HTML Mutator
		</button>
		<button
			class="py-2 px-1 border-b-2 font-medium text-sm transition-colors duration-200 {activeTab === 'encode' ? 'border-cta-blue text-cta-blue' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300'}"
			on:click={() => (activeTab = 'encode')}
		>
			Text Encoder
		</button>
	</nav>
</div>

<!-- Dirty Word Scanner Tab -->
{#if activeTab === 'scan'}
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
		<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-1">Dirty Word Scanner</h3>
		<p class="text-sm text-gray-500 dark:text-gray-400 mb-4">Scan email content for spam trigger words that may cause delivery issues.</p>
		<div class="space-y-4">
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Email Content (HTML or plain text)</label>
				<textarea
					bind:value={scanContent}
					rows="8"
					class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm font-mono"
					placeholder="Paste your email content here to scan for spam trigger words..."
				></textarea>
			</div>

			<button on:click={scanDirtyWords} disabled={loading || !scanContent} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
				{loading ? 'Scanning...' : 'Scan Content'}
			</button>

			{#if scanResults}
				<div class="mt-4 p-4 rounded-lg {scanResults.score > 5 ? 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800' : scanResults.score > 2 ? 'bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800' : 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800'}">
					<h4 class="font-semibold mb-2 text-gray-800 dark:text-gray-200">
						Spam Score: {scanResults.score}/10
						{#if scanResults.score > 5}
							<span class="text-red-600 dark:text-red-400 ml-2">High Risk</span>
						{:else if scanResults.score > 2}
							<span class="text-yellow-600 dark:text-yellow-400 ml-2">Medium Risk</span>
						{:else}
							<span class="text-green-600 dark:text-green-400 ml-2">Low Risk</span>
						{/if}
					</h4>
					{#if scanResults.matches && scanResults.matches.length > 0}
						<p class="text-sm text-gray-600 dark:text-gray-400 mb-2">Found {scanResults.matches.length} trigger word(s):</p>
						<div class="flex flex-wrap gap-2">
							{#each scanResults.matches as match}
								<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 dark:bg-red-900/40 text-red-800 dark:text-red-300">
									{match.word} ({match.category})
								</span>
							{/each}
						</div>
					{:else}
						<p class="text-sm text-green-700 dark:text-green-400">No spam trigger words found.</p>
					{/if}
				</div>
			{/if}
		</div>
	</div>
{/if}

<!-- HTML Mutator Tab -->
{#if activeTab === 'mutate'}
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
		<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-1">HTML Mutator</h3>
		<p class="text-sm text-gray-500 dark:text-gray-400 mb-4">Apply anti-fingerprinting mutations to HTML content to evade email security scanners.</p>
		<div class="space-y-4">
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Mutation Method</label>
				<select bind:value={mutationMethod} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
					{#each mutationMethods as method}
						<option value={method.value}>{method.label}</option>
					{/each}
				</select>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Intensity: {mutationIntensity}</label>
				<input type="range" min="0" max="1" step="0.1" bind:value={mutationIntensity} class="w-full" />
			</div>

			<div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Input HTML</label>
					<textarea
						bind:value={htmlInput}
						rows="10"
						class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm font-mono"
						placeholder="<html>...</html>"
					></textarea>
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Mutated Output</label>
					<textarea
						value={htmlOutput}
						rows="10"
						readonly
						class="w-full rounded-md border-gray-300 bg-gray-50 dark:bg-gray-900 dark:border-gray-600 dark:text-gray-300 shadow-sm sm:text-sm font-mono"
					></textarea>
				</div>
			</div>

			<div class="flex gap-2">
				<button on:click={mutateHTML} disabled={loading || !htmlInput} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
					{loading ? 'Mutating...' : 'Mutate HTML'}
				</button>
				{#if htmlOutput}
					<button on:click={() => copyToClipboard(htmlOutput)} class="bg-gray-500 px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200">
						Copy Output
					</button>
				{/if}
			</div>
		</div>
	</div>
{/if}

<!-- Text Encoder Tab -->
{#if activeTab === 'encode'}
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
		<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-1">Text Encoder</h3>
		<p class="text-sm text-gray-500 dark:text-gray-400 mb-4">Encode text using homoglyphs and unicode confusables to bypass keyword-based filters.</p>
		<div class="space-y-4">
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Encoding Method</label>
				<select bind:value={encodingMethod} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
					{#each encodingMethods as method}
						<option value={method.value}>{method.label}</option>
					{/each}
				</select>
			</div>

			<div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Input Text</label>
					<textarea
						bind:value={textInput}
						rows="6"
						class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
						placeholder="Enter text to encode..."
					></textarea>
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Encoded Output</label>
					<textarea
						value={textOutput}
						rows="6"
						readonly
						class="w-full rounded-md border-gray-300 bg-gray-50 dark:bg-gray-900 dark:border-gray-600 dark:text-gray-300 shadow-sm sm:text-sm"
					></textarea>
				</div>
			</div>

			<div class="flex gap-2">
				<button on:click={encodeText} disabled={loading || !textInput} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
					{loading ? 'Encoding...' : 'Encode Text'}
				</button>
				{#if textOutput}
					<button on:click={() => copyToClipboard(textOutput)} class="bg-gray-500 px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200">
						Copy Output
					</button>
				{/if}
			</div>
		</div>
	</div>
{/if}
