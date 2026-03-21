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

	let formValues = {
		enabled: false,
		siteKey: '',
		secretKey: ''
	};

	onMount(async () => {
		showIsLoading();
		try {
			const res = await api.turnstile.getSettings();
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
			const res = await api.turnstile.saveSettings(formValues);
			if (res.success) {
				addToast('Turnstile settings saved', 'Success');
			} else {
				formError = res.error || 'Failed to save settings';
			}
		} catch (e) {
			formError = 'An error occurred while saving settings';
		}
		isSubmitting = false;
	};
</script>

<HeadTitle title="Cloudflare Turnstile" />

{#if isLoaded}
	<Headline title="Cloudflare Turnstile" subtitle="Pre-lure bot verification to block automated scanners and security crawlers before they reach the proxy." />

	<div class="mb-6 p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800">
		<h3 class="text-sm font-semibold text-blue-800 dark:text-blue-300 mb-1">How it works</h3>
		<p class="text-sm text-blue-700 dark:text-blue-400">
			When enabled, visitors must pass a Cloudflare Turnstile challenge before being redirected to the phishing proxy.
			This filters out automated security scanners, headless browsers, and bot traffic that could flag your domains.
			Get your keys from <a href="https://dash.cloudflare.com/turnstile" target="_blank" class="underline font-medium">Cloudflare Dashboard</a>.
		</p>
	</div>

	<Form on:submit={onSubmit}>
		<FormGrid>
			<FormColumns>
				<FormColumn>
					<div class="flex items-center gap-3 mb-4">
						<label class="relative inline-flex items-center cursor-pointer">
							<input type="checkbox" bind:checked={formValues.enabled} class="sr-only peer" />
							<div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
							<span class="ml-3 text-sm font-medium text-gray-900 dark:text-gray-300">Enable Turnstile Pre-Lure Verification</span>
						</label>
					</div>
				</FormColumn>
			</FormColumns>

			<FormColumns>
				<FormColumn>
					<TextField
						label="Site Key"
						placeholder="0x4AAAAAAA..."
						bind:value={formValues.siteKey}
						helpText="The public site key from your Cloudflare Turnstile widget."
					/>
				</FormColumn>
				<FormColumn>
					<PasswordField
						label="Secret Key"
						placeholder="0x4AAAAAAA..."
						bind:value={formValues.secretKey}
						helpText="The secret key for server-side verification."
					/>
				</FormColumn>
			</FormColumns>

			<FormError error={formError} />

			<FormFooter>
				<FormButton type="submit" disabled={isSubmitting}>
					{isSubmitting ? 'Saving...' : 'Save Settings'}
				</FormButton>
			</FormFooter>
		</FormGrid>
	</Form>
{/if}
