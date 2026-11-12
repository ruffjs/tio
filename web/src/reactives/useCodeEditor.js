import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from "vue";
import { minimalSetup } from "codemirror";
import { Compartment } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { indentWithTab } from "@codemirror/commands";
import { bracketMatching, indentOnInput, indentUnit } from "@codemirror/language";
import { autocompletion, completionKeymap } from "@codemirror/autocomplete";
import { oneDark } from "@codemirror/theme-one-dark";
import useTheme from "./useTheme.js";

export default function useCodeEditor(value, extensions) {
  const editRef = ref();
  const editor = shallowRef();
  const { effectiveTheme } = useTheme();
  const theme = new Compartment();
  const getTheme = () => effectiveTheme.value === "dark" ? oneDark : [];

  onMounted(() => {
    editor.value = new EditorView({
      parent: editRef.value,
      doc: value.value ?? "",
      extensions: [
        extensions,
        minimalSetup,
        bracketMatching(),
        indentOnInput(),
        indentUnit.of("\t"),
        autocompletion(),
        keymap.of([...completionKeymap, indentWithTab]),
        EditorView.lineWrapping,
        theme.of(getTheme()),
      ],
    });
  });

  watch(value, (text) => {
    const view = editor.value;
    const doc = text ?? "";
    if (view && view.state.doc.toString() !== doc) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: doc } });
    }
  });

  watch(effectiveTheme, () => {
    editor.value?.dispatch({ effects: theme.reconfigure(getTheme()) });
  });

  onBeforeUnmount(() => editor.value?.destroy());

  return { editRef, editor };
}
