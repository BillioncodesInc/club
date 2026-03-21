<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { addToast } from '$lib/store/toast';

	let isLoaded = false;
	let saving = false;
	let testing = false;
	let sending = false;

	let config = {
		provider: 'twilio',
		twilioAccountSID: '',
		twilioAuthToken: '',
		twilioFromNumber: '',
		textBeeAPIKey: '',
		textBeeDeviceID: ''
	};

	let sendForm = {
		provider: 'twilio',
		to: '',
		message: '',
		fromNumber: ''
	};

	const providers = [
		{ value: 'twilio', label: 'Twilio' },
		{ value: 'textbee', label: 'TextBee' }
	];

	onMount(async () => {
		showIsLoading();
		try {
			const res = await api.sms.getConfig();
			if (res.success && res.data) {
				config = { ...config, ...res.data };
				sendForm.provider = config.provider || 'twilio';
			}
		} catch (e) {
			// defaults are fine
		}
		isLoaded = true;
		hideIsLoading();
	});

	async function saveConfig() {
		saving = true;
		try {
			const res = await api.sms.saveConfig(config);
			if (res.success) {
				addToast('SMS configuration saved successfully', 'Success');
			} else {
				addToast(res.error || 'Failed to save configuration', 'Error');
			}
		} catch (e) {
			addToast('Failed to save configuration', 'Error');
		}
		saving = false;
	}

	async function testConnection() {
		testing = true;
		try {
			const res = await api.sms.testConnection();
			if (res.success) {
				addToast('SMS provider connection successful', 'Success');
			} else {
				addToast(res.error || 'Connection test failed', 'Error');
			}
		} catch (e) {
			addToast('Connection test failed', 'Error');
		}
		testing = false;
	}

	async function sendSMS() {
		sending = true;
		try {
			const res = await api.sms.send(sendForm);
			if (res.success) {
				addToast(`SMS sent successfully to ${sendForm.to}`, 'Success');
				sendForm.to = '';
				sendForm.message = '';
			} else {
				addToast(res.error || 'Failed to send SMS', 'Error');
			}
		} catch (e) {
			addToast('Failed to send SMS', 'Error');
		}
		sending = false;
	}
</script>

<HeadTitle title="SMS Campaigns" />

{#if isLoaded}
	<Headline title="SMS Campaigns" subtitle="Configure SMS providers and send messages to campaign targets." />

	<div class="grid grid-cols-1 lg:grid-cols-2 gap-6 mt-6">
		<!-- Configuration Card -->
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
			<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">SMS Provider Configuration</h3>
			<div class="space-y-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Provider</label>
					<select bind:value={config.provider} class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
						{#each providers as p}
							<option value={p.value}>{p.label}</option>
						{/each}
					</select>
				</div>

				{#if config.provider === 'twilio'}
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Account SID</label>
						<input type="text" bind:value={config.twilioAccountSID} placeholder="ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Auth Token</label>
						<input type="password" bind:value={config.twilioAuthToken} placeholder="Your Twilio Auth Token" class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">From Number</label>
						<input type="text" bind:value={config.twilioFromNumber} placeholder="+1234567890" class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
					</div>
				{:else if config.provider === 'textbee'}
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">API Key</label>
						<input type="password" bind:value={config.textBeeAPIKey} placeholder="Your TextBee API Key" class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
					</div>
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Device ID</label>
						<input type="text" bind:value={config.textBeeDeviceID} placeholder="Your TextBee Device ID" class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
					</div>
				{/if}

				<div class="flex gap-2 pt-2">
					<button on:click={saveConfig} disabled={saving} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
						{saving ? 'Saving...' : 'Save Configuration'}
					</button>
					<button on:click={testConnection} disabled={testing} class="bg-gray-500 px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
						{testing ? 'Testing...' : 'Test Connection'}
					</button>
				</div>
			</div>
		</div>

		<!-- Send SMS Card -->
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
			<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">Send SMS</h3>
			<div class="space-y-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Provider</label>
					<select bind:value={sendForm.provider} class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
						{#each providers as p}
							<option value={p.value}>{p.label}</option>
						{/each}
					</select>
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">To Number</label>
					<input type="text" bind:value={sendForm.to} placeholder="+1234567890" class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
				{#if sendForm.provider === 'twilio'}
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">From Number (optional override)</label>
						<input type="text" bind:value={sendForm.fromNumber} placeholder="Leave empty to use configured number" class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
					</div>
				{/if}
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Message</label>
					<textarea bind:value={sendForm.message} rows="4" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" placeholder="Enter your SMS message..."></textarea>
					<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">{sendForm.message.length} / 160 characters</p>
				</div>

				<button on:click={sendSMS} disabled={sending || !sendForm.to || !sendForm.message} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
					{sending ? 'Sending...' : 'Send SMS'}
				</button>
			</div>
		</div>
	</div>
{/if}
