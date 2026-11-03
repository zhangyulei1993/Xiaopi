<script setup lang="ts">
import { onLoad } from "@dcloudio/uni-app";
import { computed, reactive, ref } from "vue";
import { API_ORIGIN, request } from "../../api/client";
import {
  createProperty,
  getPublicConfig,
  listProperties,
  updateProperty,
  type Property,
} from "../../api/properties";
import { getSession, hasRole } from "../../auth/session";
import { amenityIcon } from "../../utils/amenities";

const id = ref(""),
  currentStatus = ref("draft"),
  activeSection = ref(0),
  saving = ref(false),
  loading = ref(false),
  uploadProgress = ref(""),
  images = ref<string[]>([]),
  videos = ref<string[]>([]),
  existingMedia = ref<{ type: string; url: string }[]>([]);
const form = reactive<Partial<Property>>({
  title: "",
  community: "",
  location: "",
  latitude: 0,
  longitude: 0,
  layout: "",
  rentMonthly: 0,
  feeText: "直租无中介费",
  description: "",
  ownerId: "xiaopi-admin",
});
const roomCount = ref("一室"),
  hallCount = ref("一厅"),
  rentType = ref("整租"),
  roomType = ref("单间"),
  toiletType = ref("独卫"),
  bayWindow = ref("带飘窗");
const amenities = ref(["冰箱", "洗衣机", "空调", "热水器"]),
  amenityOptions = [
    "冰箱",
    "洗衣机",
    "空调",
    "热水器",
    "电视",
    "宽带",
    "燃气",
    "油烟机",
    "沙发",
    "衣柜",
    "书桌",
    "床",
  ];
const rooms = ["一室", "两室", "三室", "四室", "五室及以上"],
  halls = ["一厅", "两厅", "三厅", "无厅"],
  rentTypes = ["整租", "合租"],
  roomTypes = ["单间", "主卧", "次卧", "床位"],
  toilets = ["独卫", "非独卫"],
  windows = ["带飘窗", "不带飘窗"];
type FeaturedChannel = {
  title: string;
  keyword: string;
  latitude?: number;
  longitude?: number;
  radiusKm?: number;
};
const financialHarbor = {
  title: "金融港",
  keyword: "金融港",
  latitude: 30.45604,
  longitude: 114.425337,
  radiusKm: 10,
};
const channels = ref<FeaturedChannel[]>([financialHarbor]);
const selectedChannelIndex = ref(0);
const selectedChannel = computed(
  () => channels.value[selectedChannelIndex.value] || financialHarbor,
);
const channelLatitude = computed(
  () => Number(selectedChannel.value.latitude) || financialHarbor.latitude,
);
const channelLongitude = computed(
  () => Number(selectedChannel.value.longitude) || financialHarbor.longitude,
);
const channelRadiusKm = computed(
  () => Number(selectedChannel.value.radiusKm) || financialHarbor.radiusKm,
);
const channelTitles = computed(() =>
  channels.value.map(
    (channel) => `${channel.title} · ${Number(channel.radiusKm) || 5}公里`,
  ),
);
const requiredReady = computed(() =>
  Boolean(
    form.title?.trim() &&
    form.community?.trim() &&
    form.location?.trim() &&
    form.layout?.trim() &&
    Number(form.rentMonthly) > 0,
  ),
);
const sectionSteps = ["位置租金", "户型方式", "配套描述", "房源素材"];
const goToSection = (index: number) => {
  activeSection.value = Math.max(0, Math.min(sectionSteps.length - 1, index));
  uni.pageScrollTo({ scrollTop: 0, duration: 220 });
};
const geoReady = computed(() =>
  Boolean(Number(form.latitude) && Number(form.longitude)),
);
const mapLatitude = computed(() => channelLatitude.value);
const mapLongitude = computed(() => channelLongitude.value);
const channelCircles = computed(() => [
  {
    latitude: channelLatitude.value,
    longitude: channelLongitude.value,
    radius: channelRadiusKm.value * 1000,
    color: "#0E8177CC",
    fillColor: "#0E81772B",
    strokeWidth: 2,
  },
]);
const mapMarkers = computed(() =>
  geoReady.value
    ? [
        {
          id: 1,
          latitude: Number(form.latitude),
          longitude: Number(form.longitude),
          width: 30,
          height: 30,
          callout: {
            content: form.community || form.location || "房源位置",
            display: "ALWAYS",
            padding: 6,
            borderRadius: 6,
            color: "#173f3c",
            bgColor: "#ffffff",
          },
        },
      ]
    : [],
);
const distanceKm = (
  latitude1: number,
  longitude1: number,
  latitude2: number,
  longitude2: number,
) => {
  const toRadians = (value: number) => (value * Math.PI) / 180;
  const latitudeDelta = toRadians(latitude2 - latitude1);
  const longitudeDelta = toRadians(longitude2 - longitude1);
  const a =
    Math.sin(latitudeDelta / 2) ** 2 +
    Math.cos(toRadians(latitude1)) *
      Math.cos(toRadians(latitude2)) *
      Math.sin(longitudeDelta / 2) ** 2;
  return 6371 * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
};
const propertyDistance = computed(() =>
  geoReady.value
    ? distanceKm(
        channelLatitude.value,
        channelLongitude.value,
        Number(form.latitude),
        Number(form.longitude),
      )
    : 0,
);
const propertyInChannel = computed(
  () => geoReady.value && propertyDistance.value <= channelRadiusKm.value,
);
const loadChannels = async () => {
  try {
    const config = await getPublicConfig();
    if (config.featuredChannels?.length) {
      channels.value = config.featuredChannels;
      const index = channels.value.findIndex((channel) =>
        channel.title.includes("金融港"),
      );
      selectedChannelIndex.value = Math.max(0, index);
    }
  } catch {
    channels.value = [financialHarbor];
  }
};
const selectPointOnMap = (event: {
  detail?: { latitude?: number; longitude?: number };
}) => {
  const latitude = Number(event.detail?.latitude);
  const longitude = Number(event.detail?.longitude);
  if (!latitude || !longitude) return;
  form.latitude = Number(latitude.toFixed(7));
  form.longitude = Number(longitude.toFixed(7));
};
const choosePropertyLocation = () => {
  uni.chooseLocation({
    latitude: geoReady.value ? Number(form.latitude) : channelLatitude.value,
    longitude: geoReady.value ? Number(form.longitude) : channelLongitude.value,
    success: (result) => {
      form.latitude = Number(result.latitude.toFixed(7));
      form.longitude = Number(result.longitude.toFixed(7));
      form.location = result.address || result.name || form.location;
      if (!form.community?.trim() && result.name) form.community = result.name;
    },
    fail: (error) => {
      if (!String(error.errMsg || "").includes("cancel")) {
        uni.showToast({ title: "无法打开地图，请检查定位权限", icon: "none" });
      }
    },
  });
};
const mediaCount = computed(
  () =>
    existingMedia.value.filter((item) => item.type === "image").length +
    images.value.length,
);
const sectionReady = computed(() => [
  Boolean(
    form.title?.trim() &&
    form.community?.trim() &&
    form.location?.trim() &&
    Number(form.rentMonthly) > 0 &&
    geoReady.value,
  ),
  Boolean(form.layout?.trim()),
  amenities.value.length > 0,
  mediaCount.value > 0,
]);
const completionCount = computed(
  () => sectionReady.value.filter(Boolean).length,
);
const syncLayout = () => {
  form.layout = `${roomCount.value}${hallCount.value} · ${rentType.value} · ${roomType.value} · ${toiletType.value} · ${bayWindow.value}`;
};
const select = (values: string[], target: { value: string }, index: number) => {
  target.value = values[index];
  syncLayout();
};
const parseLayout = (value = "") => {
  const parts = value.split(" · ");
  if (parts.length === 5) {
    const roomHall = parts[0];
    const hall = roomHall.match(/(一厅|两厅|三厅|无厅)$/)?.[0] || "一厅";
    roomCount.value = roomHall.slice(0, -hall.length) || "一室";
    hallCount.value = hall;
    rentType.value = parts[1];
    roomType.value = parts[2];
    toiletType.value = parts[3];
    bayWindow.value = parts[4];
  } else if (parts.length >= 6) {
    roomCount.value = parts[0];
    hallCount.value = parts[1];
    rentType.value = parts[2];
    roomType.value = parts[3];
    toiletType.value = parts[4];
    bayWindow.value = parts[5];
  }
};
const loadProperty = async () => {
  if (!id.value) return;
  loading.value = true;
  try {
    const property = (await listProperties()).find(
      (item) => item.id === id.value,
    );
    if (!property) throw new Error("房源不存在");
    Object.assign(form, property);
    currentStatus.value = property.status;
    existingMedia.value = property.media || [];
    parseLayout(property.layout);
    const match = property.description.match(/^配套设施：([^\n]*)\n?/);
    amenities.value = match?.[1] ? match[1].split("、").filter(Boolean) : [];
    form.description = property.description.replace(/^配套设施：[^\n]*\n?/, "");
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "加载失败",
      icon: "none",
    });
  } finally {
    loading.value = false;
  }
};
onLoad((query) => {
  if (!hasRole("admin")) {
    uni.showToast({ title: "暂无运营权限", icon: "none" });
    uni.switchTab({ url: "/pages/me/index" });
    return;
  }
  id.value = String(query?.id || "");
  syncLayout();
  loadChannels();
  loadProperty();
});
const pick = (type: "image" | "video") => {
  const count =
    type === "image"
      ? Math.max(0, 9 - mediaCount.value)
      : 1 - videos.value.length;
  if (count <= 0) {
    uni.showToast({
      title: type === "image" ? "最多上传 9 张图片" : "最多上传 1 个视频",
      icon: "none",
    });
    return;
  }
  uni.chooseMedia({
    count,
    mediaType: [type],
    sourceType: ["album", "camera"],
    maxDuration: 60,
    sizeType: ["compressed"],
    success: (res) => {
      const paths = res.tempFiles.map((item: any) => item.tempFilePath);
      if (type === "image") images.value.push(...paths);
      else videos.value = paths.slice(0, 1);
    },
  });
};
const upload = (path: string, type: string, propertyId: string) =>
  new Promise<void>((resolve, reject) =>
    uni.uploadFile({
      url: `${API_ORIGIN}/api/v1/media/upload`,
      filePath: path,
      name: "file",
      header: { Authorization: `Bearer ${getSession()?.accessToken || ""}` },
      formData: { propertyId, mediaType: type },
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) resolve();
        else reject(new Error("素材上传失败"));
      },
      fail: reject,
    }),
  );
const save = async (submitReview = false) => {
  if (saving.value) return;
  if (!requiredReady.value) {
    uni.showToast({
      title: "请完成标题、小区、位置、户型和月租",
      icon: "none",
    });
    return;
  }
  if (submitReview && !["draft", "offline"].includes(currentStatus.value)) {
    uni.showToast({ title: "当前状态无需重复提交审核", icon: "none" });
    return;
  }
  saving.value = true;
  uploadProgress.value = "保存房源信息";
  try {
    const details = String(form.description || "").trim();
    const payload = {
      ...form,
      title: form.title?.trim(),
      community: form.community?.trim(),
      location: form.location?.trim(),
      description: `配套设施：${amenities.value.join("、")}\n${details}`.trim(),
    };
    const property = id.value
      ? await updateProperty({ ...payload, id: id.value })
      : await createProperty(payload);
    id.value = property.id;
    const queue = [
      ...images.value.map((path) => ({ path, type: "image" })),
      ...videos.value.map((path) => ({ path, type: "video" })),
    ];
    for (let index = 0; index < queue.length; index++) {
      uploadProgress.value = `上传素材 ${index + 1}/${queue.length}`;
      await upload(queue[index].path, queue[index].type, property.id);
    }
    images.value = [];
    videos.value = [];
    if (submitReview) {
      uploadProgress.value = "提交审核";
      await request("/v1/properties/status", {
        method: "PATCH",
        data: {
          propertyId: property.id,
          status: "pending_review",
          note: "管理员录入后提交审核",
        },
      });
    }
    uni.showModal({
      title: submitReview ? "已提交审核" : "草稿已保存",
      content: submitReview
        ? "可在工作台审核并发布。"
        : "可继续补充素材，稍后再提交审核。",
      showCancel: false,
      success: () => submitReview && uni.navigateBack(),
    });
  } catch (error) {
    uni.showToast({
      title: error instanceof Error ? error.message : "保存失败",
      icon: "none",
    });
  } finally {
    saving.value = false;
    uploadProgress.value = "";
  }
};
const mediaUrl = (url: string) =>
  url.startsWith("http") ? url : `${API_ORIGIN}${url}`;
</script>
<template>
  <view class="page"
    ><view class="editor-hero"
      ><view
        ><text>{{ id ? "编辑房源" : "录入新房源" }}</text
        ><small>按步骤完善信息，随时可以保存草稿</small></view
      ><view class="completion"
        ><strong>{{ completionCount }}/4</strong><small>已完成</small></view
      ></view
    ><scroll-view class="step-nav" scroll-x :show-scrollbar="false"
      ><view class="step-track"
        ><view
          v-for="(step, index) in sectionSteps"
          :key="step"
          class="step-item"
          :class="{
            active: activeSection === index,
            done: sectionReady[index],
          }"
          @click="goToSection(index)"
          ><text>{{ sectionReady[index] ? "✓" : index + 1 }}</text
          ><small>{{ step }}</small></view
        ></view
      ></scroll-view
    ><view v-if="loading" class="card loading">正在加载房源...</view
    ><template v-else
      ><view v-show="activeSection === 0" class="section-card"
        ><view class="section-head"
          ><text>01</text
          ><view
            ><strong>基础信息</strong
            ><small>用户最先看到的标题、位置和租金</small></view
          ></view
        ><view class="field"
          ><text>房源标题 *</text
          ><input
            v-model.trim="form.title"
            maxlength="40"
            placeholder="例如：金融港地铁口采光一居室" /></view
        ><view class="field"
          ><text>小区名称 *</text
          ><input
            v-model.trim="form.community"
            maxlength="40"
            placeholder="请输入真实小区名称" /></view
        ><view class="field"
          ><text>位置描述 *</text
          ><input
            v-model.trim="form.location"
            maxlength="60"
            placeholder="区域 / 地铁站 / 商圈" /></view
        ><view class="channel-scope"
          ><text>精选频道关联范围</text
          ><picker
            :range="channelTitles"
            :value="selectedChannelIndex"
            @change="selectedChannelIndex = Number($event.detail.value)"
            ><view class="channel-picker"
              ><view
                ><strong>{{ selectedChannel.title }}</strong
                ><small
                  >以频道中心为圆心，半径 {{ channelRadiusKm }} 公里</small
                ></view
              ><text>切换 ›</text></view
            ></picker
          ></view
        ><view class="map-picker"
          ><map
            class="property-map"
            :latitude="mapLatitude"
            :longitude="mapLongitude"
            :markers="mapMarkers"
            :circles="channelCircles"
            :scale="11"
            show-location
            @tap="selectPointOnMap"
          /><view class="map-picker-footer"
            ><view
              ><strong :class="{ warning: geoReady && !propertyInChannel }">{{
                geoReady
                  ? propertyInChannel
                    ? `已在${selectedChannel.title}范围内`
                    : `超出${selectedChannel.title}范围`
                  : `请在${selectedChannel.title}圆形范围内选点`
              }}</strong
              ><small v-if="geoReady"
                >距频道中心 {{ propertyDistance.toFixed(2) }} 公里 ·
                {{ Number(form.latitude).toFixed(6) }},
                {{ Number(form.longitude).toFixed(6) }}</small
              ><small v-else
                >直接点击地图即可落点，也可以搜索小区或详细地址</small
              ></view
            ><button size="mini" @click="choosePropertyLocation">
              搜索地址
            </button></view
          ></view
        ><view class="rent-row"
          ><view class="field"
            ><text>月租 *</text
            ><view class="rent-input"
              ><input
                v-model.number="form.rentMonthly"
                type="number"
                placeholder="0"
              /><small>元/月</small></view
            ></view
          ><view class="field"
            ><text>费用说明</text
            ><input
              v-model.trim="form.feeText"
              maxlength="30"
              placeholder="押一付一" /></view></view></view
      ><view v-show="activeSection === 1" class="section-card"
        ><view class="section-head"
          ><text>02</text
          ><view
            ><strong>户型与租住方式</strong
            ><small>点击每一项调整，系统自动生成户型摘要</small></view
          ></view
        ><view class="picker-grid"
          ><picker
            :range="rooms"
            @change="select(rooms, roomCount, $event.detail.value)"
            ><view class="picker-item"
              ><strong>{{ roomCount }}</strong
              ><small>卧室</small></view
            ></picker
          ><picker
            :range="halls"
            @change="select(halls, hallCount, $event.detail.value)"
            ><view class="picker-item"
              ><strong>{{ hallCount }}</strong
              ><small>客厅</small></view
            ></picker
          ><picker
            :range="rentTypes"
            @change="select(rentTypes, rentType, $event.detail.value)"
            ><view class="picker-item"
              ><strong>{{ rentType }}</strong
              ><small>方式</small></view
            ></picker
          ><picker
            :range="roomTypes"
            @change="select(roomTypes, roomType, $event.detail.value)"
            ><view class="picker-item"
              ><strong>{{ roomType }}</strong
              ><small>房间</small></view
            ></picker
          ><picker
            :range="toilets"
            @change="select(toilets, toiletType, $event.detail.value)"
            ><view class="picker-item"
              ><strong>{{ toiletType }}</strong
              ><small>卫生间</small></view
            ></picker
          ><picker
            :range="windows"
            @change="select(windows, bayWindow, $event.detail.value)"
            ><view class="picker-item"
              ><strong>{{ bayWindow }}</strong
              ><small>窗型</small></view
            ></picker
          ></view
        ><view class="layout-preview"
          ><text>房源摘要</text><strong>{{ form.layout }}</strong></view
        ></view
      ><view v-show="activeSection === 2" class="section-card"
        ><view class="section-head"
          ><text>03</text
          ><view
            ><strong>配套与描述</strong
            ><small>准确描述可减少重复咨询</small></view
          ></view
        ><checkbox-group
          class="amenity-grid"
          @change="amenities = $event.detail.value"
          ><label
            v-for="option in amenityOptions"
            :key="option"
            :class="{ active: amenities.includes(option) }"
            ><checkbox
              :value="option"
              :checked="amenities.includes(option)"
              color="#0e8177"
            /><image
              class="amenity-option-icon"
              :src="amenityIcon(option)"
              mode="aspectFit"
            /><text>{{ option }}</text></label
          ></checkbox-group
        ><view class="description">
          <textarea
            v-model.trim="form.description"
            maxlength="800"
            placeholder="填写采光、楼层、朝向、入住要求、周边交通等信息"
          ></textarea
          ><text>{{ String(form.description || "").length }}/800</text></view
        ></view
      ><view v-show="activeSection === 3" class="section-card"
        ><view class="section-head"
          ><text>04</text
          ><view
            ><strong>图片与视频</strong
            ><small>建议首图展示客厅或卧室全景，最多 9 图 1 视频</small></view
          ></view
        ><view
          v-if="existingMedia.length || images.length || videos.length"
          class="media-preview"
          ><view
            v-for="(media, index) in existingMedia"
            :key="media.url"
            class="preview-item"
            ><image
              v-if="media.type === 'image'"
              :src="mediaUrl(media.url)"
              mode="aspectFill"
            /><video v-else :src="mediaUrl(media.url)" /><text
              >已上传 {{ index + 1 }}</text
            ></view
          ><view
            v-for="(path, index) in images"
            :key="path"
            class="preview-item"
            ><image :src="path" mode="aspectFill" /><text
              class="remove"
              @click="images.splice(index, 1)"
              >×</text
            ></view
          ><view
            v-for="(path, index) in videos"
            :key="path"
            class="preview-item"
            ><video :src="path" /><text
              class="remove"
              @click="videos.splice(index, 1)"
              >×</text
            ></view
          ></view
        ><view class="media-actions"
          ><view @click="pick('image')"
            ><strong>＋ 添加图片</strong><small>{{ mediaCount }}/9</small></view
          ><view @click="pick('video')"
            ><strong>＋ 添加视频</strong
            ><small>{{ videos.length }}/1 · 最长60秒</small></view
          ></view
        ></view
      ><view class="save-space" /><view class="save-bar"
        ><view v-if="saving" class="progress">{{ uploadProgress }}</view
        ><view v-if="activeSection < 3" class="step-actions"
          ><button
            v-if="activeSection > 0"
            class="back-button"
            @click="goToSection(activeSection - 1)"
          >
            上一步</button
          ><view v-else class="draft-link" @click="save(false)">保存草稿</view
          ><button
            class="primary next-button"
            @click="goToSection(activeSection + 1)"
          >
            下一步 · {{ sectionSteps[activeSection + 1] }}
          </button></view
        ><view v-else class="save-actions"
          ><button :disabled="saving" @click="save(false)">保存草稿</button
          ><button
            class="primary"
            :disabled="
              saving ||
              !requiredReady ||
              !['draft', 'offline'].includes(currentStatus)
            "
            @click="save(true)"
          >
            {{
              saving
                ? "处理中..."
                : currentStatus === "pending_review"
                  ? "已在审核中"
                  : currentStatus === "published"
                    ? "已发布"
                    : "保存并提交审核"
            }}
          </button></view
        ></view
      ></template
    ></view
  >
</template>
<style scoped>
.editor-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  margin: -28rpx -32rpx 0;
  padding: 38rpx 32rpx 64rpx;
  border-radius: 0 0 44rpx 44rpx;
  background:
    radial-gradient(
      circle at 88% 10%,
      rgba(229, 196, 133, 0.22),
      transparent 38%
    ),
    #173f3c;
  color: #fff;
}
.editor-hero view:first-child > text,
.editor-hero small {
  display: block;
}
.editor-hero view:first-child > text {
  font-size: 42rpx;
  font-weight: 800;
  letter-spacing: -1rpx;
}
.editor-hero small {
  margin-top: 10rpx;
  color: rgba(255, 255, 255, 0.68);
  font-size: 22rpx;
  line-height: 1.55;
}
.completion {
  flex: none;
  text-align: right;
}
.completion strong,
.completion small {
  display: block;
}
.completion strong {
  color: #f0cf8d;
  font-size: 36rpx;
  font-variant-numeric: tabular-nums;
}
.completion small {
  margin-top: 2rpx;
  color: rgba(255, 255, 255, 0.58);
  font-size: 19rpx;
}
.step-nav {
  position: relative;
  z-index: 2;
  width: auto;
  margin: -36rpx 0 24rpx;
  border-radius: 24rpx;
  background: rgba(255, 255, 255, 0.97);
  box-shadow: 0 18rpx 42rpx rgba(23, 63, 60, 0.12);
}
.step-track {
  display: flex;
  min-width: 680rpx;
  padding: 14rpx 10rpx;
}
.step-item {
  display: flex;
  min-width: 150rpx;
  flex: 1;
  align-items: center;
  gap: 8rpx;
  padding: 12rpx 10rpx;
  color: #8aa7a0;
}
.step-item text {
  display: flex;
  width: 34rpx;
  height: 34rpx;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #edf4f1;
  font-size: 19rpx;
  font-weight: 700;
}
.step-item small {
  font-size: 20rpx;
  white-space: nowrap;
}
.step-item.active {
  color: #173f3c;
  font-weight: 700;
}
.step-item.active text {
  background: #173f3c;
  color: #fff;
}
.step-item.done:not(.active) text {
  background: #dcefe8;
  color: #0e8177;
}
.loading {
  text-align: center;
  color: #78a39d;
}
.section-card {
  margin-bottom: 20rpx;
  padding: 34rpx 30rpx 40rpx;
  border-radius: 26rpx;
  background: #fff;
  box-shadow: 0 12rpx 36rpx rgba(23, 63, 60, 0.055);
}
.section-head {
  display: flex;
  align-items: center;
  gap: 18rpx;
  margin-bottom: 18rpx;
}
.section-head > text {
  display: flex;
  width: 52rpx;
  height: 52rpx;
  align-items: center;
  justify-content: center;
  border-radius: 14rpx;
  background: #edf5f2;
  color: #0e8177;
  font-size: 21rpx;
  font-weight: 800;
}
.section-head strong,
.section-head small {
  display: block;
}
.section-head strong {
  color: #173f3c;
  font-size: 30rpx;
}
.section-head small {
  margin-top: 5rpx;
  color: #78a39d;
  font-size: 21rpx;
}
.field {
  margin-top: 18rpx;
  padding: 20rpx 22rpx;
  border: 1rpx solid #e3eeea;
  border-radius: 16rpx;
  background: #fbfdfc;
}
.field > text {
  display: block;
  margin-bottom: 12rpx;
  color: #6d8e89;
  font-size: 21rpx;
  font-weight: 600;
}
.field input {
  width: 100%;
  color: #073b3a;
  font-size: 27rpx;
}
.rent-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14rpx;
}
.channel-scope {
  margin-top: 28rpx;
}
.channel-scope > text {
  display: block;
  margin-bottom: 12rpx;
  color: #4f7974;
  font-size: 24rpx;
  font-weight: 700;
}
.channel-picker {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20rpx;
  border: 0;
  border-radius: 16rpx;
  background: #edf6f3;
}
.channel-picker strong,
.channel-picker small {
  display: block;
}
.channel-picker strong {
  color: #173f3c;
  font-size: 25rpx;
}
.channel-picker small {
  margin-top: 6rpx;
  color: #78a39d;
  font-size: 20rpx;
}
.channel-picker > text {
  color: #0e8177;
  font-size: 22rpx;
  font-weight: 700;
}
.map-picker {
  margin: 14rpx 0 8rpx;
  overflow: hidden;
  border: 1rpx solid #dfeae6;
  border-radius: 20rpx;
  background: #f4fbf8;
}
.property-map {
  width: 100%;
  height: 330rpx;
}
.map-picker-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18rpx;
  padding: 20rpx;
}
.map-picker-footer > view {
  min-width: 0;
  flex: 1;
}
.map-picker-footer strong,
.map-picker-footer small {
  display: block;
}
.map-picker-footer strong {
  color: #173f3c;
  font-size: 24rpx;
}
.map-picker-footer strong.warning {
  color: #c36b2b;
}
.map-picker-footer small {
  margin-top: 6rpx;
  color: #78a39d;
  font-size: 20rpx;
  line-height: 1.45;
}
.map-picker-footer button {
  flex: none;
  margin: 0;
  border: 0;
  border-radius: 12rpx;
  background: #0e8177;
  color: #fff;
  font-size: 22rpx;
}
.rent-input {
  display: flex;
  align-items: center;
}
.rent-input input {
  min-width: 0;
  flex: 1;
}
.rent-input small {
  flex: none;
  color: #8aa7a0;
  font-size: 21rpx;
}
.picker-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 14rpx;
}
.picker-item {
  position: relative;
  padding: 24rpx 22rpx;
  text-align: left;
  border: 1rpx solid #e1ece8;
  border-radius: 16rpx;
  background: #fbfdfc;
}
.picker-item strong,
.picker-item small {
  display: block;
}
.picker-item strong {
  color: #173f3c;
  font-size: 28rpx;
}
.picker-item small {
  margin-top: 6rpx;
  color: #86a19c;
  font-size: 20rpx;
}
.layout-preview {
  margin-top: 22rpx;
  padding: 22rpx;
  border-left: 6rpx solid #d9b876;
  border-radius: 4rpx 16rpx 16rpx 4rpx;
  background: #f7f4ed;
}
.layout-preview text,
.layout-preview strong {
  display: block;
}
.layout-preview text {
  color: #78a39d;
  font-size: 21rpx;
}
.layout-preview strong {
  margin-top: 7rpx;
  color: #315f5a;
  font-size: 24rpx;
}
.amenity-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12rpx;
}
.amenity-grid label {
  display: flex;
  min-height: 132rpx;
  box-sizing: border-box;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8rpx;
  padding: 14rpx 8rpx 12rpx;
  border: 1rpx solid #e2eeea;
  border-radius: 16rpx;
  color: #5f8580;
  font-size: 23rpx;
  text-align: center;
}
.amenity-grid label.active {
  border-color: #9dd8cc;
  background: #edfaf6;
  color: #0e8177;
}
.amenity-grid checkbox {
  position: absolute;
  width: 1rpx;
  height: 1rpx;
  opacity: 0;
}
.amenity-option-icon {
  width: 48rpx;
  height: 48rpx;
  padding: 10rpx;
  border-radius: 18rpx;
  background: #f0f8f5;
}
.amenity-grid label.active .amenity-option-icon {
  background: #d9f2ea;
}
.description {
  position: relative;
  margin-top: 22rpx;
  padding: 18rpx;
  border: 1rpx solid #e2ece9;
  border-radius: 16rpx;
  background: #fbfdfc;
}
.description textarea {
  width: 100%;
  height: 190rpx;
  font-size: 26rpx;
}
.description > text {
  position: absolute;
  right: 16rpx;
  bottom: 12rpx;
  color: #9ab2ae;
  font-size: 19rpx;
}
.media-preview {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12rpx;
  margin-bottom: 18rpx;
}
.preview-item {
  position: relative;
  height: 160rpx;
  overflow: hidden;
  border-radius: 16rpx;
  background: #e6f3ef;
}
.preview-item image,
.preview-item video {
  width: 100%;
  height: 100%;
}
.preview-item > text {
  position: absolute;
  left: 8rpx;
  bottom: 8rpx;
  padding: 4rpx 8rpx;
  border-radius: 8rpx;
  background: rgba(0, 0, 0, 0.5);
  color: #fff;
  font-size: 18rpx;
}
.preview-item > .remove {
  left: auto;
  right: 8rpx;
  top: 8rpx;
  bottom: auto;
  width: 38rpx;
  height: 38rpx;
  padding: 0;
  border-radius: 50%;
  text-align: center;
  font-size: 30rpx;
}
.media-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14rpx;
}
.media-actions > view {
  padding: 34rpx 12rpx;
  border: 2rpx dashed #b7dcd4;
  border-radius: 16rpx;
  text-align: center;
}
.media-actions strong,
.media-actions small {
  display: block;
}
.media-actions strong {
  color: #0e8177;
  font-size: 24rpx;
}
.media-actions small {
  margin-top: 8rpx;
  color: #78a39d;
  font-size: 20rpx;
}
.save-space {
  height: 150rpx;
}
.save-bar {
  position: fixed;
  z-index: 5;
  left: 0;
  right: 0;
  bottom: 0;
  padding: 16rpx 32rpx calc(16rpx + env(safe-area-inset-bottom));
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 -8rpx 28rpx rgba(15, 70, 65, 0.09);
}
.progress {
  margin-bottom: 10rpx;
  color: #0e8177;
  font-size: 21rpx;
  text-align: center;
}
.save-actions {
  display: grid;
  grid-template-columns: 0.8fr 1.2fr;
  gap: 14rpx;
}
.step-actions {
  display: grid;
  grid-template-columns: minmax(150rpx, 0.7fr) minmax(0, 1.5fr);
  align-items: center;
  gap: 14rpx;
}
.step-actions button,
.save-actions button {
  height: 88rpx;
  line-height: 88rpx;
}
.draft-link {
  color: #5f807b;
  font-size: 25rpx;
  font-weight: 600;
  text-align: center;
}
.back-button {
  margin: 0;
  border: 0;
  border-radius: 16rpx;
  background: #edf4f1;
  color: #315f5a;
  font-size: 25rpx;
}
.next-button {
  margin: 0;
  border: 0;
  border-radius: 16rpx;
  background: #173f3c;
  color: #fff;
  font-size: 26rpx;
  font-weight: 700;
}
.next-button:active,
.back-button:active,
.media-actions > view:active,
.picker-item:active {
  transform: scale(0.985);
}
.save-actions button {
  margin: 0;
  border: 0;
  border-radius: 16rpx;
  background: #edf6f3;
  color: #0e8177;
  font-size: 27rpx;
}
.save-actions button::after {
  border: 0;
}
.save-actions .primary {
  background: #0e8177;
  color: #fff;
}
.save-actions button[disabled] {
  opacity: 0.5;
}
</style>
