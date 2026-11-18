import { computed, ref, watch } from "vue";
import { request } from "../../api/client";

export function usePropertyDoorplate(form: { community?: string; floorInfo?: string }) {
  const buildingNo = ref("");
  const unitNo = ref("");
  const roomNo = ref("");
  const selectedFloor = ref("");
  type Doorplate = { building: string; unit: string; room: string };
  const buildingRules = ref<Doorplate[]>([]);
  const manualDoorplate = ref("");
  const doorplateEntryMode = ref<"catalog" | "manual">("catalog");
  type StructureStep = "building" | "unit" | "room" | "";
  const structureStep = ref<StructureStep>("");

  const buildingOptions = computed(() => [...new Set(buildingRules.value.map(row => row.building))]);
  const hasDoorplateCatalog = computed(() => buildingOptions.value.length > 0);
  const automaticFloorInfo = computed(() => selectedFloor.value ? `${selectedFloor.value}楼` : "");
  const buildingRows = computed(() => buildingRules.value.filter(row => row.building === buildingNo.value));
  const unitOptions = computed(() => [...new Set(buildingRows.value.map(row => row.unit.replace(/单元$/, "")))].filter(Boolean));
  const roomOptions = computed(() => [...new Set(buildingRows.value
    .filter(row => row.unit.replace(/单元$/, "") === unitNo.value)
    .map(row => row.room))].map(roomNo => ({ roomNo, floor: /^\d{3,4}$/.test(roomNo) ? roomNo.slice(0, -2) : "" })));
  const generatedRoomNo = computed(() => {
    return roomNo.value;
  });
  const loadBuildingRules = async () => {
    const community = String(form.community || "").trim();
    if (!community) {
      buildingRules.value = [];
      return;
    }
    buildingRules.value = [];
    const rows = await request<Doorplate[]>(`/v1/admin/doorplates?community=${encodeURIComponent(community)}`).catch(() => []);
    if (String(form.community || "").trim() === community) buildingRules.value = rows;
  };
  const selectBuildingRule = (building: string) => {
    buildingNo.value = building;
    unitNo.value = "";
    selectedFloor.value = "";
    roomNo.value = "";
    form.floorInfo = "";
    structureStep.value = "unit";
  };
  const chooseUnit = (unit: string) => {
    unitNo.value = unit;
    selectedFloor.value = "";
    roomNo.value = "";
    form.floorInfo = "";
    structureStep.value = "room";
  };
  const chooseRoom = (room: { floor: string; roomNo: string }) => {
    selectedFloor.value = room.floor;
    roomNo.value = room.roomNo;
    form.floorInfo = automaticFloorInfo.value;
    structureStep.value = "";
  };
  const structureStepIndex = (step: StructureStep) =>
    (["building", "unit", "room"] as StructureStep[]).indexOf(step);
  const canReturnToStructureStep = (step: StructureStep) => {
    if (step === "building") return true;
    if (step === "unit") return Boolean(buildingNo.value);
    return Boolean(buildingNo.value && unitNo.value);
  };
  const returnToStructureStep = (step: StructureStep) => {
    if (!step || !canReturnToStructureStep(step)) return;
    if (step === "building") {
      unitNo.value = "";
      selectedFloor.value = "";
      roomNo.value = "";
      form.floorInfo = "";
    } else if (step === "unit") {
      roomNo.value = "";
      selectedFloor.value = "";
      form.floorInfo = "";
    }
    structureStep.value = step;
  };
  const returnToPreviousStructureStep = () => {
    const currentIndex = structureStepIndex(structureStep.value);
    if (currentIndex <= 0) return;
    returnToStructureStep((["building", "unit", "room"] as StructureStep[])[currentIndex - 1]);
  };
  const openStructurePicker = async () => {
    if (!form.community?.trim()) return;
    await loadBuildingRules();
    // Default to the catalog when first opened; keep the operator's last choice
    // on subsequent opens. Communities without catalog data always use manual entry.
    if (!hasDoorplateCatalog.value) doorplateEntryMode.value = "manual";
    structureStep.value = buildingNo.value
      ? unitNo.value
        ? "room"
        : "unit"
      : "building";
  };
  const switchDoorplateEntryMode = (mode: "catalog" | "manual") => {
    if (mode === "catalog" && !hasDoorplateCatalog.value) return;
    manualDoorplate.value = "";
    doorplateEntryMode.value = mode;
  };
  const structureSummary = computed(() =>
    [buildingNo.value, unitNo.value && `${unitNo.value}单元`, roomNo.value && `${roomNo.value}室`]
      .filter(Boolean)
      .join("") || "未选择",
  );
  watch(structureStep, () => { manualDoorplate.value = ""; });
  const confirmManualDoorplate = () => {
    const value = manualDoorplate.value.trim();
    if (!value) return;
    if (structureStep.value === "building") selectBuildingRule(/^\d+$/.test(value) ? `${value}栋` : value);
    else if (structureStep.value === "unit") chooseUnit(value.replace(/单元$/, ""));
    else chooseRoom({ roomNo: value.replace(/室$/, ""), floor: /^\d{3,4}$/.test(value) ? value.slice(0, -2) : "" });
  };

  return {
    buildingNo,
    unitNo,
    roomNo,
    selectedFloor,
    buildingRules,
    manualDoorplate,
    doorplateEntryMode,
    structureStep,
    buildingOptions,
    hasDoorplateCatalog,
    automaticFloorInfo,
    unitOptions,
    roomOptions,
    generatedRoomNo,
    loadBuildingRules,
    selectBuildingRule,
    chooseUnit,
    chooseRoom,
    canReturnToStructureStep,
    returnToStructureStep,
    returnToPreviousStructureStep,
    openStructurePicker,
    switchDoorplateEntryMode,
    structureSummary,
    confirmManualDoorplate,
  };
}
