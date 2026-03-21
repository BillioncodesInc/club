<script>
	import HeadTitle from './HeadTitle.svelte';
	import Headline from './Headline.svelte';
	export let title = '';
</script>
<HeadTitle {title} />
<Headline>{title}</Headline>
