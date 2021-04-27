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
      editor: null
    };
  },
  mounted() {
    console.log("Options", this.$props.options);
    this.editor = new Quill(this.$refs.editor, this.$props.options);

    this.editor.root.innerHTML = this.value;

    this.editor.on('text-change', () => this.update());
  },

  methods: {
    update() {
      this.$emit('input', this.editor.getText() ? this.editor.root.innerHTML : '');
    }
  }
}
</script>

<template>
  <div class="quill-editor">
    <slot name="toolbar"></slot>
    <div ref="editor" v-html="value"></div>
  </div>

</template>