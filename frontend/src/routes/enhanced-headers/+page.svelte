<script>
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { addToast } from '$lib/store/toast';

	let loading = false;

	let fromDomain = '';
	let fromEmail = '';
	let toEmail = '';
	let selectedProfile = 'all';

	let profiles = [];

	const profileOptions = [
		{ value: 'all', label: 'All Profiles' },
		{ value: 'exchange', label: 'Microsoft Exchange / O365' },
		{ value: 'google', label: 'Google Workspace / Gmail' },
		{ value: 'generic', label: 'Generic SMTP' }
	];

	async function generateHeaders() {
		loading = true;
		profiles = [];
		try {
			const res = await api.enhancedHeaders.generate(fromDomain, fromEmail, toEmail, selectedProfile);
			if (res.success && res.data) {
				if (Array.isArray(res.data)) {
					profiles = res.data;
				} else {
					profiles = [res.data];
				}
			} else {
				addToast(res.error || 'Failed to generate headers', 'Error');
			}
		} catch (e) {
			addToast('Failed to generate headers', 'Error');
		}
		loading = false;
	}

	function copyHeaders(headers) {
		const text = Object.entries(headers)
			.map(([key, value]) => `${key}: ${value}`)
			.join('\n');
		navigator.clipboard.writeText(text);
		addToast('Headers copied to clipboard', 'Success');
	}

	function copyAsJSON(headers) {
		navigator.clipboard.writeText(JSON.stringify(headers, null, 2));
		addToast('Headers JSON copied to clipboard', 'Success');
	}
</script>

<HeadTitle title="Enhanced Email Headers" />
<Headline title="Enhanced Email Headers" subtitle="Generate realistic email headers that mimic legitimate mail servers to improve deliverability." />

<div class="space-y-6 mt-6">
	<!-- Generator Form -->
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
		<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-1">Header Generator</h3>
		<p class="text-sm text-gray-500 dark:text-gray-400 mb-4">Copy the generated headers into your SMTP Configuration's custom headers.</p>
		<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">From Domain</label>
				<input type="text" bind:value={fromDomain} placeholder="example.com" class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
			</div>
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">From Email</label>
				<input type="text" bind:value={fromEmail} placeholder="sender@example.com" class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
			</div>
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">To Email (optional)</label>
				<input type="text" bind:value={toEmail} placeholder="recipient@target.com" class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
			</div>
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Profile</label>
				<select bind:value={selectedProfile} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
					{#each profileOptions as opt}
						<option value={opt.value}>{opt.label}</option>
					{/each}
				</select>
			</div>
		</div>
		<div class="mt-4">
			<button on:click={generateHeaders} disabled={loading || !fromDomain || !fromEmail} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
				{loading ? 'Generating...' : 'Generate Headers'}
			</button>
		</div>
	</div>

	<!-- Generated Profiles -->
	{#each profiles as profile}
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
			<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-1">{profile.name}</h3>
			<p class="text-sm text-gray-500 dark:text-gray-400 mb-4">{profile.description}</p>
			<div class="overflow-x-auto">
				<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
					<thead class="bg-gray-50 dark:bg-gray-900/40">
						<tr>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase w-1/3">Header</th>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Value</th>
						</tr>
					</thead>
					<tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
						{#each Object.entries(profile.headers) as [key, value]}
							<tr>
								<td class="px-4 py-2 text-sm font-mono font-medium text-gray-900 dark:text-gray-200 whitespace-nowrap">{key}</td>
								<td class="px-4 py-2 text-sm font-mono text-gray-500 dark:text-gray-400 break-all">{value || '(empty)'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			<div class="mt-4 flex gap-2">
				<button on:click={() => copyHeaders(profile.headers)} class="bg-gray-500 px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200">
					Copy as Text
				</button>
				<button on:click={() => copyAsJSON(profile.headers)} class="bg-gray-500 px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200">
					Copy as JSON
				</button>
			</div>
		</div>
	{/each}
</div>
