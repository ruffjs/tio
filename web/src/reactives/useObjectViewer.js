import { ref } from "vue";

export default () => {
  const objectToBeView = ref(null);
  const titleOfViewer = ref("");
  const viewObjectAsTree = ref(false);
  const viewObject = (obj, type, asTree = false) => {
    // console.log(obj, type);
    objectToBeView.value = obj;
    titleOfViewer.value = type;
    viewObjectAsTree.value = !!asTree;
  };
  const handleCloseViewer = () => {
    objectToBeView.value = null;
    titleOfViewer.value = "";
    viewObjectAsTree.value = false;
  };
  return {
    objectToBeView,
    titleOfViewer,
    viewObjectAsTree,
    viewObject,
    handleCloseViewer,
  };
};
