<script>
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { addToast } from '$lib/store/toast';

	let loading = false;

	let targetVolume = 500;
	let isNewDomain = true;
	let hasReputation = false;

	let schedule = null;
	let config = null;

	async function calculateSchedule() {
		loading = true;
		try {
			const res = await api.emailWarming.calculateSchedule(targetVolume, isNewDomain, hasReputation);
			if (res.success && res.data) {
				schedule = res.data.schedule;
				config = res.data.config;
			} else {
				addToast(res.error || 'Failed to calculate schedule', 'Error');
			}
		} catch (e) {
			addToast('Failed to calculate schedule', 'Error');
		}
		loading = false;
	}
</script>

<HeadTitle title="Email Warming" />
<Headline title="Email Warming" subtitle="Calculate optimal email warming schedules for new domains." />

<div class="grid grid-cols-1 lg:grid-cols-3 gap-6 mt-6">
	<!-- Configuration Card -->
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200 lg:col-span-1">
		<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">Warming Calculator</h3>
		<div class="space-y-4">
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Target Daily Volume</label>
				<input type="number" bind:value={targetVolume} placeholder="500" class="block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
			</div>

			<div>
				<label class="flex items-center gap-2">
					<input type="checkbox" bind:checked={isNewDomain} class="rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
					<span class="text-sm font-medium text-gray-700 dark:text-gray-300">New domain (no sending history)</span>
				</label>
			</div>

			<div>
				<label class="flex items-center gap-2">
					<input type="checkbox" bind:checked={hasReputation} class="rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
					<span class="text-sm font-medium text-gray-700 dark:text-gray-300">Has existing sender reputation</span>
				</label>
			</div>

			<button on:click={calculateSchedule} disabled={loading} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
				{loading ? 'Calculating...' : 'Calculate Schedule'}
			</button>

			{#if config}
				<div class="mt-4 p-3 bg-gray-50 dark:bg-gray-900/40 rounded-lg text-sm">
					<h4 class="font-semibold mb-2 text-gray-700 dark:text-gray-200">Recommended Configuration</h4>
					<dl class="space-y-1">
						<div class="flex justify-between">
							<dt class="text-gray-500 dark:text-gray-400">Start Volume:</dt>
							<dd class="font-medium text-gray-800 dark:text-gray-200">{config.startVolume}/day</dd>
						</div>
						<div class="flex justify-between">
							<dt class="text-gray-500 dark:text-gray-400">Max Volume:</dt>
							<dd class="font-medium text-gray-800 dark:text-gray-200">{config.maxVolume}/day</dd>
						</div>
						<div class="flex justify-between">
							<dt class="text-gray-500 dark:text-gray-400">Growth Rate:</dt>
							<dd class="font-medium text-gray-800 dark:text-gray-200">{config.growthRate}x daily</dd>
						</div>
						<div class="flex justify-between">
							<dt class="text-gray-500 dark:text-gray-400">Duration:</dt>
							<dd class="font-medium text-gray-800 dark:text-gray-200">{config.daysToComplete} days</dd>
						</div>
					</dl>
				</div>
			{/if}
		</div>
	</div>

	<!-- Schedule Preview -->
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200 lg:col-span-2">
		<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">Warming Schedule</h3>
		{#if schedule && schedule.length > 0}
			<div class="overflow-x-auto">
				<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
					<thead class="bg-gray-50 dark:bg-gray-900/40">
						<tr>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Day</th>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Volume</th>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Interval</th>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Progress</th>
						</tr>
					</thead>
					<tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
						{#each schedule as day}
							<tr>
								<td class="px-4 py-2 text-sm font-medium text-gray-900 dark:text-gray-200">Day {day.day}</td>
								<td class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400">{day.volume} emails</td>
								<td class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400">{day.interval} between sends</td>
								<td class="px-4 py-2">
									<div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
										<div
											class="bg-cta-blue h-2 rounded-full"
											style="width: {Math.min((day.volume / targetVolume) * 100, 100)}%"
										></div>
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<!-- Visual Chart -->
			<div class="mt-6 p-4 bg-gray-50 dark:bg-gray-900/40 rounded-lg">
				<h4 class="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">Volume Ramp-Up</h4>
				<div class="flex items-end gap-1 h-32">
					{#each schedule as day}
						<div
							class="bg-cta-blue rounded-t flex-1 min-w-[8px] transition-all hover:opacity-80"
							style="height: {Math.max((day.volume / targetVolume) * 100, 3)}%"
							title="Day {day.day}: {day.volume} emails"
						></div>
					{/each}
				</div>
				<div class="flex justify-between text-xs text-gray-400 dark:text-gray-500 mt-1">
					<span>Day 1</span>
					<span>Day {schedule.length}</span>
				</div>
			</div>
		{:else}
			<div class="text-center py-12 text-gray-500 dark:text-gray-400">
				<p>Configure your warming parameters and click "Calculate Schedule" to see the recommended warming plan.</p>
			</div>
		{/if}
	</div>
</div>
