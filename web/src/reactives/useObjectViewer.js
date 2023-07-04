import { ref } from "vue";

export default () => {
  const selectedObject = ref(null);
  const selectedType = ref("");
  const viewObject = (obj, type) => {
    // console.log(obj, type);
    selectedObject.value = obj;
    selectedType.value = type;
  };
  const handleCloseViewer = () => {
    selectedObject.value = null;
    selectedType.value = "";
  };
  return { selectedObject, selectedType, viewObject, handleCloseViewer };
};
