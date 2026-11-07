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
	// ruleid: ts-no-inner-html
	el['innerHTML'] = name;
	// ruleid: ts-no-inner-html
	el['outerHTML'] += name;
	// ruleid: ts-no-inner-html
	Object.assign(el, { innerHTML: name });
	// ruleid: ts-no-inner-html
	document.createRange().createContextualFragment(name);
	// ruleid: ts-no-inner-html
	el.setHTMLUnsafe(name);
	// ruleid: ts-no-inner-html
	Document.parseHTMLUnsafe(name);
}

function safe(el: HTMLElement, name: string): void {
	// ok: ts-no-inner-html
	el.textContent = name;
	// ok: ts-no-inner-html
	el.append(document.createTextNode(name));
	// ok: ts-no-inner-html
	const html = el.innerHTML.length;
	void html;
	// ok: ts-no-inner-html
	Object.assign(el, { title: name });
	// ok: ts-no-inner-html
	el['title'] = name;
}
