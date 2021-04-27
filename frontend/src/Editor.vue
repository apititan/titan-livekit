<script>
// https://pineco.de/wrapping-quill-editor-in-a-vue-component/

import Quill from 'quill';

export default {
  props: {
    value: {
      type: String,
      default: ''
    },
    options: {
      type: Object
    }
  },

  data() {
    return {
      editor: null,
      cachedValue: null
    };
  },
  beforeMount() {
    this.cachedValue = this.$props.value;
  },
  mounted() {
    this.editor = new Quill(this.$refs.editor, this.$props.options);

    this.editor.root.innerHTML = this.cachedValue;

    this.editor.on('text-change', () => this.onUpdate());
  },

  methods: {
    onUpdate() {
      let html = this.editor.getText() ? this.editor.root.innerHTML : '';
      if (html === '<p><br></p>') html = '';
      this.$emit('input', html);
    },
    clear() {
      this.editor.deleteText(0, this.editor.getLength());
    }
  }
}
</script>

<template>
  <div class="quill-editor">
    <slot name="toolbar"></slot>
    <div ref="editor" v-html="cachedValue"></div>
  </div>

</template>