import { reactive } from 'vue';
import { ElNotification } from 'element-plus';
import * as ruleSchema from '@/components/rule/rule-schema';
import { deepCopy } from '@/utils/common';

/**
 * Create a component editor
 * @param {Object} config 
 * @param {Function} onConfigChange 
 * @returns {Object} Component editor
 */
export function createComponentEditor(config, onConfigChange) {
  // edit state
  const drawerEdit = reactive({
    show: false,
    new: true,
    type: '',
    title: '',
    schema: [],
    optionsSchema: {},
    data: {},
    parentEditor: null, // for nested edit (e.g. add Connector when editing Source)
    callback: null, // callback after edit
  });

  // Reset edit state
  const resetEdit = () => {
    drawerEdit.show = false;
    drawerEdit.new = true;
    drawerEdit.type = '';
    drawerEdit.title = '';
    drawerEdit.schema = [];
    drawerEdit.optionsSchema = {};
    drawerEdit.data = {};
    drawerEdit.parentEditor = null;
    drawerEdit.callback = null;
  };

  // Show add Connector interface
  const showAddConnector = (parentEditor = null, callback = null) => {
    drawerEdit.type = 'connector';
    drawerEdit.new = true;
    drawerEdit.title = "Add Connector";
    drawerEdit.schema = deepCopy(ruleSchema.connector);
    drawerEdit.data = deepCopy(ruleSchema.defaultConnector);
    drawerEdit.optionsSchema = deepCopy(ruleSchema.connectorOptions);
    drawerEdit.parentEditor = parentEditor;
    drawerEdit.callback = callback;
    drawerEdit.show = true;
  };

  // Edit Connector
  const editConnector = (row, parentEditor = null, callback = null) => {
    drawerEdit.type = 'connector';
    drawerEdit.new = false;
    drawerEdit.title = "Edit Connector";
    drawerEdit.schema = deepCopy(ruleSchema.connector);
    drawerEdit.data = deepCopy(row);
    drawerEdit.optionsSchema = deepCopy(ruleSchema.connectorOptions);
    drawerEdit.parentEditor = parentEditor;
    drawerEdit.callback = callback;
    drawerEdit.show = true;
  };

  // Show add Source interface
  const showAddSource = (parentEditor = null, callback = null) => {
    drawerEdit.type = 'source';
    drawerEdit.new = true;
    drawerEdit.title = "Add Source";
    drawerEdit.schema = deepCopy(ruleSchema.source);
    drawerEdit.data = deepCopy(ruleSchema.defaultSrc);
    drawerEdit.optionsSchema = deepCopy(ruleSchema.sourceOptions);
    drawerEdit.parentEditor = parentEditor;
    drawerEdit.callback = callback;
    drawerEdit.show = true;
  };

  // Edit Source
  const editSource = (row, parentEditor = null, callback = null) => {
    drawerEdit.type = 'source';
    drawerEdit.new = false;
    drawerEdit.title = "Edit Source";
    drawerEdit.schema = deepCopy(ruleSchema.source);
    drawerEdit.data = deepCopy(row);
    drawerEdit.optionsSchema = deepCopy(ruleSchema.sourceOptions);
    drawerEdit.parentEditor = parentEditor;
    drawerEdit.callback = callback;
    drawerEdit.show = true;
  };

  // Show add Sink interface
  const showAddSink = (parentEditor = null, callback = null) => {
    drawerEdit.type = 'sink';
    drawerEdit.new = true;
    drawerEdit.title = "Add Sink";
    drawerEdit.schema = deepCopy(ruleSchema.sink);
    drawerEdit.data = deepCopy(ruleSchema.defaultSink);
    drawerEdit.optionsSchema = deepCopy(ruleSchema.sinkOptions);
    drawerEdit.parentEditor = parentEditor;
    drawerEdit.callback = callback;
    drawerEdit.show = true;
  };

  // Edit Sink
  const editSink = (row, parentEditor = null, callback = null) => {
    drawerEdit.type = 'sink';
    drawerEdit.new = false;
    drawerEdit.title = "Edit Sink";
    drawerEdit.schema = deepCopy(ruleSchema.sink);
    drawerEdit.data = deepCopy(row);
    drawerEdit.optionsSchema = deepCopy(ruleSchema.sinkOptions);
    drawerEdit.parentEditor = parentEditor;
    drawerEdit.callback = callback;
    drawerEdit.show = true;
  };

  // Confirm edit
  const confirmEdit = async (editOptRef) => {
    const d = deepCopy(drawerEdit.data);
    const v = await editOptRef.value.validate();
    if (!v.valid) return false;

    const localConfig = deepCopy(config);
    if (!localConfig[drawerEdit.type + 's']) {
      localConfig[drawerEdit.type + 's'] = [];
    }

    const list = localConfig[drawerEdit.type + 's'];
    const exist = list.find(c => c.name === d.name);
    
    if (exist && drawerEdit.new) {
      ElNotification({ type: 'error', message: 'Name already exists!' });
      return false;
    }
    
    if (drawerEdit.new) {
      d.enabled = true;
      list.push(d);
    } else {
      for (let i = 0; i < list.length; i++) {
        if (list[i].name === d.name) {
          list[i] = d;
          break;
        }
      }
    }

    // If there is a parent editor and callback, execute the callback
    if (drawerEdit.parentEditor && drawerEdit.callback) {
      drawerEdit.callback(d);
      drawerEdit.parentEditor.show = true; // Show parent editor again
    } else {
      // Otherwise update the global config
      onConfigChange(localConfig);
    }

    resetEdit();
    return true;
  };

  // Cancel edit
  const cancelEdit = () => {
    // If there is a parent editor, show the parent editor again
    if (drawerEdit.parentEditor) {
      drawerEdit.parentEditor.show = true;
    }
    resetEdit();
  };

  return {
    drawerEdit,
    showAddConnector,
    editConnector,
    showAddSource,
    editSource,
    showAddSink,
    editSink,
    confirmEdit,
    cancelEdit
  };
}
