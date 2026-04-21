<script>
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { addToast } from '$lib/store/toast';

	let strategy = 'weighted';
	let contentVariants = [
		{ id: 'v1', subject: '', body: '', weight: 1 }
	];
	let senderVariants = [];
	let recipientText = '';
	let spinTemplate = '';
	let spinResult = '';

	let balanceResult = null;
	let balancing = false;
	let spinning = false;

	function addContentVariant() {
		const id = 'v' + (contentVariants.length + 1);
		contentVariants = [...contentVariants, { id, subject: '', body: '', weight: 1 }];
	}

	function removeContentVariant(index) {
		contentVariants = contentVariants.filter((_, i) => i !== index);
	}

	function addSenderVariant() {
		const id = 's' + (senderVariants.length + 1);
		senderVariants = [...senderVariants, { id, fromName: '', fromEmail: '', replyTo: '', weight: 1, rateLimit: 50, delayMs: 500 }];
	}

	function removeSenderVariant(index) {
		senderVariants = senderVariants.filter((_, i) => i !== index);
	}

	function parseRecipients(text) {
		return text.split('\n').filter(l => l.trim()).map(line => {
			const parts = line.split(',').map(p => p.trim());
			return {
				email: parts[0] || '',
				firstName: parts[1] || '',
				lastName: parts[2] || ''
			};
		});
	}

	async function balance() {
		balancing = true;
		balanceResult = null;
		try {
			const recipients = parseRecipients(recipientText);
			if (recipients.length === 0) {
				addToast('Please add at least one recipient', 'Error');
				balancing = false;
				return;
			}
			const req = {
				recipients,
				contentVariants,
				senderVariants: senderVariants.length > 0 ? senderVariants : undefined,
				strategy
			};
			const res = await api.contentBalancer.balance(req);
			if (res && res.data) {
				balanceResult = res.data;
				addToast('Balanced ' + balanceResult.totalRecipients + ' recipients across ' + Object.keys(balanceResult.variantCounts).length + ' variants', 'Success');
			}
		} catch (e) {
			addToast('Balance failed: ' + (e.message || 'Unknown error'), 'Error');
		}
		balancing = false;
	}

	async function spin() {
		spinning = true;
		spinResult = '';
		try {
			const res = await api.contentBalancer.spin(spinTemplate);
			if (res && res.data) {
				spinResult = res.data.result;
			}
		} catch (e) {
			spinResult = 'Error: ' + (e.message || 'Unknown error');
		}
		spinning = false;
	}
</script>

<HeadTitle title="Content Balancer" />
<Headline title="Content Balancer" subtitle="Distribute recipients across multiple email content variants and sender identities to avoid fingerprinting." />

<div class="grid grid-cols-1 lg:grid-cols-2 gap-6 mt-6">
	<!-- Left: Configuration -->
	<div class="space-y-6">
		<!-- Strategy -->
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
			<h2 class="font-semibold text-gray-700 dark:text-gray-200 mb-3">Distribution Strategy</h2>
			<select bind:value={strategy} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
				<option value="weighted">Weighted (distribute by weight ratio)</option>
				<option value="round_robin">Round Robin (alternate evenly)</option>
				<option value="by_domain">By Domain (same domain = same variant)</option>
				<option value="random">Random</option>
			</select>
		</div>

		<!-- Content Variants -->
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
			<div class="flex justify-between items-center mb-3">
				<h2 class="font-semibold text-gray-700 dark:text-gray-200">Content Variants</h2>
				<button on:click={addContentVariant} class="text-sm text-cta-blue hover:underline">+ Add Variant</button>
			</div>

			{#each contentVariants as variant, i}
				<div class="border border-gray-200 dark:border-gray-600 rounded p-3 mb-3">
					<div class="flex justify-between items-center mb-2">
						<span class="text-sm font-medium text-gray-600 dark:text-gray-400">Variant {i + 1}</span>
						{#if contentVariants.length > 1}
							<button on:click={() => removeContentVariant(i)} class="text-xs text-red-500 hover:underline">Remove</button>
						{/if}
					</div>
					<input type="text" bind:value={variant.subject} placeholder="Subject line"
						class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm mb-2" />
					<textarea bind:value={variant.body} placeholder="Email body (HTML or text)" rows="3"
						class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm mb-2"></textarea>
					<div class="flex items-center gap-2">
						<label class="text-xs text-gray-500 dark:text-gray-400">Weight:</label>
						<input type="number" bind:value={variant.weight} min="1" max="100"
							class="w-16 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
					</div>
				</div>
			{/each}
		</div>

		<!-- Sender Variants (optional) -->
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
			<div class="flex justify-between items-center mb-3">
				<h2 class="font-semibold text-gray-700 dark:text-gray-200">Sender Variants <span class="text-xs text-gray-600 dark:text-gray-400">(optional)</span></h2>
				<button on:click={addSenderVariant} class="text-sm text-cta-blue hover:underline">+ Add Sender</button>
			</div>

			{#each senderVariants as sender, i}
				<div class="border border-gray-200 dark:border-gray-600 rounded p-3 mb-3">
					<div class="flex justify-between items-center mb-2">
						<span class="text-sm font-medium text-gray-600 dark:text-gray-400">Sender {i + 1}</span>
						<button on:click={() => removeSenderVariant(i)} class="text-xs text-red-500 hover:underline">Remove</button>
					</div>
					<div class="grid grid-cols-2 gap-2 text-sm">
						<input type="text" bind:value={sender.fromName} placeholder="From Name" class="rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						<input type="text" bind:value={sender.fromEmail} placeholder="From Email" class="rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						<input type="number" bind:value={sender.rateLimit} placeholder="Rate limit/hr" class="rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						<input type="number" bind:value={sender.delayMs} placeholder="Delay (ms)" class="rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
					</div>
				</div>
			{/each}
			{#if senderVariants.length === 0}
				<p class="text-sm text-gray-400 dark:text-gray-500">No sender variants. Recipients will use the default campaign sender.</p>
			{/if}
		</div>
	</div>

	<!-- Right: Recipients & Results -->
	<div class="space-y-6">
		<!-- Recipients -->
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
			<h2 class="font-semibold text-gray-700 dark:text-gray-200 mb-3">Recipients</h2>
			<p class="text-xs text-gray-500 dark:text-gray-400 mb-2">One per line: email, firstName, lastName</p>
			<textarea bind:value={recipientText} rows="8"
				class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm font-mono text-xs"
				placeholder="john@example.com, John, Doe&#10;jane@corp.com, Jane, Smith&#10;bob@company.org, Bob, Wilson"></textarea>
			<div class="text-xs text-gray-400 dark:text-gray-500 mt-1">{parseRecipients(recipientText).length} recipients</div>
		</div>

		<button on:click={balance} disabled={balancing}
			class="w-full bg-cta-blue px-4 py-3 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
			{balancing ? 'Balancing...' : 'Balance Recipients'}
		</button>

		<!-- Results -->
		{#if balanceResult}
			<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
				<h2 class="font-semibold text-gray-700 dark:text-gray-200 mb-3">Distribution Result</h2>
				<div class="grid grid-cols-2 gap-3 mb-4 text-sm">
					<div class="text-gray-700 dark:text-gray-300"><span class="font-medium">Total:</span> {balanceResult.totalRecipients}</div>
					<div class="text-gray-700 dark:text-gray-300"><span class="font-medium">Strategy:</span> {balanceResult.strategy}</div>
					<div class="text-gray-700 dark:text-gray-300"><span class="font-medium">Est. Time:</span> {balanceResult.estimatedTime}</div>
				</div>

				<h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Variant Distribution</h3>
				<div class="space-y-1 mb-4">
					{#each Object.entries(balanceResult.variantCounts) as [id, count]}
						<div class="flex items-center gap-2">
							<span class="text-xs font-mono w-12 text-gray-600 dark:text-gray-400">{id}</span>
							<div class="flex-1 bg-gray-200 dark:bg-gray-700 rounded h-4">
								<div class="bg-cta-blue rounded h-4"
									style="width: {(count / balanceResult.totalRecipients * 100)}%"></div>
							</div>
							<span class="text-xs w-16 text-right text-gray-600 dark:text-gray-400">{count} ({Math.round(count / balanceResult.totalRecipients * 100)}%)</span>
						</div>
					{/each}
				</div>

				<details class="text-xs">
					<summary class="cursor-pointer text-cta-blue">View full assignments</summary>
					<pre class="mt-2 bg-gray-50 dark:bg-gray-900/40 p-2 rounded overflow-auto max-h-60 font-mono text-gray-700 dark:text-gray-300">{JSON.stringify(balanceResult.assignments, null, 2)}</pre>
				</details>
			</div>
		{/if}

		<!-- Text Spinner -->
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
			<h2 class="font-semibold text-gray-700 dark:text-gray-200 mb-3">Text Spinner</h2>
			<p class="text-xs text-gray-500 dark:text-gray-400 mb-2">Use &#123;option1|option2|option3&#125; syntax for random selection per send.</p>
			<textarea bind:value={spinTemplate} rows="3"
				class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm mb-2"
				placeholder="Hi &#123;there|friend|colleague&#125;, please &#123;review|check|see&#125; the attached &#123;document|file|report&#125;."></textarea>
			<button on:click={spin} disabled={spinning}
				class="bg-gray-500 px-3 py-1 text-white rounded-md text-sm hover:opacity-80 transition-all duration-200 disabled:opacity-50">
				{spinning ? 'Spinning...' : 'Spin Preview'}
			</button>
			{#if spinResult}
				<div class="mt-2 p-2 bg-gray-50 dark:bg-gray-900/40 rounded text-sm text-gray-700 dark:text-gray-300">{spinResult}</div>
			{/if}
		</div>
	</div>
</div>
