for (const answered of document.querySelectorAll('[data-answered-key]')) {
  try { sessionStorage.removeItem(`sense-draft:${answered.dataset.answeredKey}`); } catch (_) { /* storage unavailable */ }
}

for (const input of document.querySelectorAll('[data-draft-key]')) {
  const key = `sense-draft:${input.dataset.draftKey}`;
  try {
    const saved = sessionStorage.getItem(key);
    if (saved !== null && input.value === '') input.value = saved;
  } catch (_) { /* forms remain usable without storage */ }
  input.addEventListener('input', () => {
    try { sessionStorage.setItem(key, input.value); } catch (_) { /* storage unavailable */ }
  });
}
