function unsafe(el: HTMLElement, name: string): void {
	// ruleid: ts-no-inner-html
	el.innerHTML = name;
	// ruleid: ts-no-inner-html
	el.outerHTML = '<b>' + name + '</b>';
	// ruleid: ts-no-inner-html
	el.insertAdjacentHTML('beforeend', name);
	// ruleid: ts-no-inner-html
	document.write(name);
	// ruleid: ts-no-inner-html
	document.body.innerHTML += name;
}

function safe(el: HTMLElement, name: string): void {
	// ok: ts-no-inner-html
	el.textContent = name;
	// ok: ts-no-inner-html
	el.append(document.createTextNode(name));
	// ok: ts-no-inner-html
	const html = el.innerHTML.length;
	void html;
}
