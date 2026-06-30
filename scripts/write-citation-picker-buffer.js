#!/usr/bin/env osascript -l JavaScript

const isNode = typeof process !== "undefined" && process.release && process.release.name === "node";

let readFile, fileExists, getenv;

if (isNode) {
	const fs = require("fs");
	readFile = function (path) {
		return fs.readFileSync(path, "utf8");
	};
	fileExists = function (filePath) {
		if (!filePath) return false;
		try {
			return fs.existsSync(filePath);
		} catch (e) {
			return false;
		}
	};
	getenv = function (key) {
		return process.env[key] || "";
	};
} else {
	// JXA fallback
	ObjC.import("stdlib");
	const app = Application.currentApplication();
	app.includeStandardAdditions = true;

	readFile = function (path) {
		try {
			const data = $.NSFileManager.defaultManager.contentsAtPath(path);
			const str = $.NSString.alloc.initWithDataEncoding(data, $.NSUTF8StringEncoding);
			return ObjC.unwrap(str);
		} catch (e) {
			return "";
		}
	};
	fileExists = function (filePath) {
		if (!filePath) return false;
		try {
			return $.NSFileManager.defaultManager.fileExistsAtPath(filePath);
		} catch (e) {
			return false;
		}
	};
	getenv = function (key) {
		try {
			return $.getenv(key);
		} catch (e) {
			return "";
		}
	};
}

//──────────────────────────────────────────────────────────────────────────────

class BibtexEntry {
	constructor() {
		/** @type {string[]} */ this.author = []; // last names only
		/** @type {string[]} */ this.editor = [];
		this.icon = "";
		this.citekey = ""; // without "@"
		this.title = "";
		this.year = ""; // as string since no calculations are made
		this.origyear = ""; // as string since no calculations are made
		this.url = "";
		this.booktitle = "";
		this.journal = "";
		this.doi = "";
		this.volume = "";
		this.issue = "";
		this.abstract = "";
		/** @type {string[]} */ this.keywords = [];
		this.attachment = "";
	}

	primaryNamesArr() {
		if (this.author.length > 0) return this.author;
		return this.editor; // if both are empty, will also return empty array
	}
	/** turn Array of names into into one string to display
	 * @param {string[]} names
	 */
	etAlStringify(names) {
		switch (names.length) {
			case 0:
				return "";
			case 1:
				return names[0];
			case 2:
				return names.join(" & ");
			default:
				return names[0] + " et al.";
		}
	}

	get primaryNames() {
		return this.primaryNamesArr();
	}
	get primaryNamesEtAlString() {
		return this.etAlStringify(this.primaryNamesArr());
	}
	get authorsEtAlString() {
		return this.etAlStringify(this.author);
	}
	get editorsEtAlString() {
		return this.etAlStringify(this.editor);
	}
}

/**
 * @param {string} encodedStr
 * @return {string} decodedStr
 */
function bibtexDecode(encodedStr) {
	if (!encodedStr) return "";
	const decodePairs = {
		'{\\"u}': "ü",
		'{\\"a}': "ä",
		'{\\"o}': "ö",
		'{\\"U}': "Ü",
		'{\\"A}': "Ä",
		'{\\"O}': "Ö",
		'\\"u': "ü",
		'\\"a': "ä",
		'\\"o': "ö",
		'\\"U': "Ü",
		'\\"A': "Ä",
		'\\"O': "Ö",
		"\\ss": "ß",
		"{\\ss}": "ß",
		// bibtex-tidy
		'\\"{O}': "Ö",
		'\\"{o}': "ö",
		'\\"{A}': "Ä",
		'\\"{a}': "ä",
		'\\"{u}': "ü",
		'\\"{U}': "Ü",
		// Bookends
		"\\''A": "Ä",
		"\\''O": "Ö",
		"\\''U": "Ü",
		"\\''a": "ä",
		"\\''o": "ö",
		"\\''u": "ü",
		// frech chars
		"{\\'a}": "a",
		"{\\'o}": "ó",
		"{\\'e}": "e",
		"{\\`{e}}": "e",
		"{\\`e}": "e",
		"\\'E": "É",
		"\\c{c}": "c",
		'\\"{i}': "i",
		// other chars
		"{\\~n}": "n",
		"\\~a": "ã",
		"{\\v c}": "c",
		"\\o{}": "ø",
		"{\\o}": "ø",
		"{\\O}": "Ø",
		"\\^{i}": "i",
		"\\'\\i": "í",
		"{\\'c}": "c",
		// special chars
		"{\\ldots}": "…",
		"\\&": "&",
		"``": '"',
		",,": '"',
		"`": "'",
		"\\textendash{}": "—",
		"---": "—",
		"--": "—",
		"{extquotesingle}": "'",
		'\\"e': "e",
	};

	// Escape regex special chars
	function escapeRegExp(string) {
		return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
	}

	// Build regex sorting longest first to avoid partial matches
	const decodeRegex = new RegExp(
		Object.keys(decodePairs)
			.sort((a, b) => b.length - a.length)
			.map(escapeRegExp)
			.join("|"),
		"g"
	);

	return encodedStr.replace(decodeRegex, (match) => decodePairs[match]);
}

/**
 * @param {string} rawBibtexStr
 * @return {BibtexEntry[]}
 */
function bibtexParse(rawBibtexStr) {
	const decoded = bibtexDecode(rawBibtexStr);
	const entries = decoded.split(/^@/m).slice(1);
	const bibtexEntryArray = [];

	for (const rawEntryStr of entries) {
		const trimmed = rawEntryStr.trim();
		const openBracePos = trimmed.indexOf("{");
		if (openBracePos === -1) continue;
		const firstCommaPos = trimmed.indexOf(",", openBracePos);
		if (firstCommaPos === -1) continue;

		const category = trimmed.slice(0, openBracePos).trim();
		const citekey = trimmed.slice(openBracePos + 1, firstCommaPos).trim();
		let propertyStr = trimmed.slice(firstCommaPos + 1);
		if (propertyStr.endsWith("}")) propertyStr = propertyStr.slice(0, -1);

		const entry = new BibtexEntry();
		entry.citekey = citekey;
		entry.icon = category.toLowerCase();

		// last comma of a field as delimiter https://regex101.com/r/1dvpfC/1
		const properties = propertyStr.trim().split(/,(?=\s*[\w-]+\s*=)/);

		for (const line of properties) {
			const equalSignPos = line.indexOf("=");
			if (equalSignPos === -1) continue; // GUARD erroneous BibTeX formatting, empty lines, etc.

			const field = line.slice(0, equalSignPos).trim().toLowerCase();
			const value = line
				.slice(equalSignPos + 1)
				.replace(/{|}|,$/g, "") // remove TeX escaping
				.trim();

			switch (field) {
				case "author":
				case "editor": {
					// create last name array
					entry[field] = value.split(" and ").map((name) => {
						const lastname = name.includes(",")
							? name.split(",")[0] // when last name — first name
							: name.split(" ").pop(); // when first name — last name
						return lastname || "ERROR";
					});
					break;
				}
				case "date":
				case "year": {
					const yearDigits = value.match(/\d{4}/);
					if (yearDigits) entry.year = yearDigits[0]; // edge case of BibTeX files with wrong years
					break;
				}
				case "keywords": {
					entry.keywords = value ? value.split(/ *, */) : [];
					break;
				}
				case "file":
				case "local-url":
				case "attachment": {
					// PERF file is decoded later when opening
					entry.attachment = value;
					break;
				}
				default:
					// check if field is needed before adding it, to reduce JSON size
					if (field in entry) {
						// @ts-expect-error unclear how to annotate it so typescript is happy
						entry[field] = value;
					}
			}
		}

		if (!entry.url && entry.doi) entry.url = "https://doi.org/" + entry.doi;
		bibtexEntryArray.push(entry);
	}

	return bibtexEntryArray;
}

//──────────────────────────────────────────────────────────────────────────────

// biome-ignore lint/correctness/noUnusedVariables: Alfred run
function run() {
	const urlEmoji = "🌐";
	const litNoteEmoji = "📓";
	const tagEmoji = "🏷";
	const attachmentEmoji = "📎";
	const abstractEmoji = "📄";
	const pdfEmoji = "📕";
	const secondLibraryIcon = "2️⃣ ";
	const litNoteFilterStr = "*";
	const pdfFilterStr = "pdf";
	const alfredBarWidth = Number.parseInt(getenv("alfred_bar_width"));
	const alfredWorkflowData = getenv("alfred_workflow_data");

	const matchAuthorsInEtAl = getenv("match_authors_in_etal") === "1";
	const matchShortYears = getenv("match_year_type").includes("short");
	const matchFullYears = getenv("match_year_type").includes("full");
	const openEntriesIn = getenv("open_entries_in");

	const libraryPath = getenv("bibtex_library_path");
	const secondaryLibraryPath = getenv("secondary_library_path");

	const litNoteFolder = getenv("literature_note_folder");
	const pdfFolder = getenv("pdf_folder");
	const litNoteFolderExists = fileExists(litNoteFolder);
	const pdfFolderExists = fileExists(pdfFolder);

	// GUARD
	if (pdfFolder && !pdfFolderExists) {
		return JSON.stringify({
			items: [{ title: "PDF folder does not exist.", subtitle: pdfFolder, valid: false }],
		});
	}
	if (litNoteFolder && !litNoteFolderExists) {
		return JSON.stringify({
			items: [
				{ title: "Literature folder does not exist.", subtitle: litNoteFolder, valid: false },
			],
		});
	}

	//──────────────────────────────────────────────────────────────────────────────

	const pdfListFile = alfredWorkflowData + "/pdf_list.txt";
	const litListFile = alfredWorkflowData + "/lit_list.txt";

	const pdfArray = (pdfFolderExists && fileExists(pdfListFile))
		? readFile(pdfListFile).split("\n")
		: [];
	const litNoteArray = (litNoteFolderExists && fileExists(litListFile))
		? readFile(litListFile).split("\n")
		: [];

	const pdfSet = new Set(pdfArray);
	const litNoteSet = new Set(litNoteArray);

	//──────────────────────────────────────────────────────────────────────────────

	/**
	 * @param {BibtexEntry} entry
	 * @param {"first"|"second"} whichLibrary
	 */
	function convertToAlfredItems(entry, whichLibrary) {
		const emojis = [];
		// biome-ignore format: too long
		const {
			title, url, citekey, keywords, icon, journal, volume, issue, booktitle, origyear,
			author, editor, year, abstract, primaryNamesEtAlString, primaryNames, attachment
		} = entry;
		const isFirstLibrary = whichLibrary === "first";

		// Shorten Title (for display in Alfred)
		let shorterTitle = title;
		if (title.length > alfredBarWidth) shorterTitle = title.slice(0, alfredBarWidth).trim() + "…";

		// URL
		let urlSubtitle = "⛔ There is no URL or DOI.";
		if (url) {
			emojis.push(urlEmoji);
			urlSubtitle = "⌃: Open URL – " + url;
		}

		let extraMatcher = "";

		// Literature Notes
		const hasLitNote = litNoteFolderExists && litNoteSet.has(citekey);
		if (hasLitNote) {
			emojis.push(litNoteEmoji);
			extraMatcher += litNoteFilterStr;
		}

		// PDFs
		const hasPdf = pdfFolderExists && pdfSet.has(citekey);
		if (hasPdf) {
			emojis.push(pdfEmoji);
			extraMatcher += pdfFilterStr;
		}

		// Emojis
		if (abstract) emojis.push(abstractEmoji);
		if (keywords.length > 0) emojis.push(tagEmoji + " " + keywords.length.toString());
		if (attachment) emojis.push(attachmentEmoji);

		// Icon selection
		const iconPath = `icons/${icon}.png`;

		// Journal/Book Title
		let collectionSubtitle = "";
		if (icon === "article" && journal) {
			collectionSubtitle += "    In: " + journal;
			if (volume) collectionSubtitle += " " + volume;
			if (issue) collectionSubtitle += "(" + issue + ")";
		}
		if ((icon === "incollection" || icon === "inbook") && booktitle)
			collectionSubtitle += "    In: " + booktitle;

		// display editor and add "Ed." when no authors
		let namesToDisplay = primaryNamesEtAlString + " ";
		if (author.length === 0 && editor.length > 0) {
			if (editor.length > 1) namesToDisplay += "(Eds.) ";
			else namesToDisplay += "(Ed.) ";
		}

		// Matching behavior
		/** @type {string[]} */
		let keywordMatches = [];
		if (keywords.length > 0)
			keywordMatches = keywords.map((/** @type {string} */ tag) => "#" + tag);
		let authorMatches = [...author, ...editor];
		if (!matchAuthorsInEtAl) authorMatches = [...author.slice(0, 1), ...editor.slice(0, 1)]; // only match first two names
		const yearMatches = [];
		if (matchShortYears) yearMatches.push(year.slice(-2));
		if (matchFullYears) yearMatches.push(year);

		const alfredMatcher = [
			"@" + citekey,
			...keywordMatches,
			title,
			...authorMatches,
			...yearMatches,
			booktitle,
			journal,
			extraMatcher,
		]
			.filter(Boolean)
			.map((item) => item.includes("-") ? (item.replaceAll("-", " ") + " " + item) : item)
			.join(" ");

		// Alfred: Large Type
		let largeTypeInfo = `${title} \n(citekey: ${citekey})`;
		if (abstract) largeTypeInfo += "\n\n" + abstract;
		if (keywords.length > 0) largeTypeInfo += "\n\nkeywords: " + keywords.join(", ");

		// // Indicate 2nd library (this set via .map thisAry)
		const libraryIndicator = isFirstLibrary ? "" : secondLibraryIcon;

		// year
		const displayYear = origyear ? `${year} [${origyear}]` : year;

		return {
			title: libraryIndicator + shorterTitle,
			autocomplete: primaryNames[0],
			subtitle: namesToDisplay + displayYear + collectionSubtitle + "   " + emojis.join(" "),
			match: alfredMatcher,
			arg: citekey,
			icon: { path: iconPath },
			uid: citekey,
			text: {
				copy: url,
				largetype: largeTypeInfo,
			},
			quicklookurl: url,
			mods: {
				ctrl: {
					valid: url !== "",
					arg: url,
					subtitle: urlSubtitle,
				},
				shift: {
					// opening in second library not implemented yet
					valid: isFirstLibrary,
					subtitle: isFirstLibrary
						? `⇧: Open in ${openEntriesIn}`
						: "⛔: Opening entries in 2nd library not yet implemented.",
				},
				"fn+cmd": {
					valid: isFirstLibrary,
					subtitle: isFirstLibrary
						? "⌘+fn: Delete entry from BibTeX file (⚠️ irreversible)."
						: "⛔: Deleting entries in 2nd library not yet implemented.",
				},
				"ctrl+alt+cmd": {
					valid: Boolean(attachment),
					subtitle: attachment
						? "⌃⌥⌘: Open Attachment File"
						: "⛔: Entry has no attachment file.",
					arg: attachment,
				},
			},
		};
	}

	//───────────────────────────────────────────────────────────────────────────

	const firstBibtex = readFile(libraryPath);
	const firstBibtexEntryArray = bibtexParse(firstBibtex)
		.reverse() // reverse, so recent entries come first
		.map((item) => convertToAlfredItems(item, "first"));

	const secondBibtex = fileExists(secondaryLibraryPath) ? readFile(secondaryLibraryPath) : "";
	const secondBibtexEntryArray = bibtexParse(secondBibtex)
		.reverse()
		.map((item) => convertToAlfredItems(item, "second"));

	return JSON.stringify({ items: [...firstBibtexEntryArray, ...secondBibtexEntryArray] });
}

if (isNode) {
	console.log(run());
}
