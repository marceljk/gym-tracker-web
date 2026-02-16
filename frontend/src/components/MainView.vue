<template>
  <v-container class="fill-height pa-4">
    <v-row>
      <v-col cols="12" md="4">
        <v-select
          label="Days"
          :items="statDays"
          v-model="selectedDays"
          multiple
          clearable
          open-on-clear
        ></v-select>
        <v-text-field
          label="Date"
          :active="datePicker"
          :focused="datePicker"
          readonly
          v-model="selectedDate"
          clearable
          @click:clear="selectedDate = null"
        >
          <v-dialog v-model="datePicker" activator="parent" width="auto">
            <v-date-picker
              v-if="datePicker"
              @update:model-value="selectDate"
            ></v-date-picker>
          </v-dialog>
        </v-text-field>
        <v-text-field
          label="From"
          :active="timePickerFrom"
          :focused="timePickerFrom"
          readonly
          v-model="filterFrom"
          clearable
        >
          <!-- v-model="filterFrom" -->
          <v-dialog v-model="timePickerFrom" activator="parent" width="auto">
            <v-time-picker
              v-if="timePickerFrom"
              format="24hr"
              :allowed-minutes="allowedMinutes"
              @update:model-value="
                (e) => {
                  filterFrom = e;
                  timePickerFrom = false;
                }
              "
            ></v-time-picker>
          </v-dialog>
        </v-text-field>
        <v-text-field
          label="Until"
          :active="timePickerUntil"
          :focused="timePickerUntil"
          readonly
          v-model="filterUntil"
          clearable
        >
          <v-dialog v-model="timePickerUntil" activator="parent" width="auto">
            <v-time-picker
              v-if="timePickerUntil"
              format="24hr"
              :allowed-minutes="allowedMinutes"
              @update:model-value="
                (e) => {
                  filterUntil = e;
                  timePickerUntil = false;
                }
              "
            ></v-time-picker>
          </v-dialog>
        </v-text-field>
        <v-card>
          <div class="d-flex justify-space-between align-center">
            <v-card-title>Live</v-card-title>
            <v-btn
              :loading="isUpdatingVisitors"
              class="mr-3"
              variant="text"
              prepend-icon="mdi-reload"
              @click="refreshCurrentVisitors"
            >
              Refresh
            </v-btn>
          </div>
          <span class="d-flex justify-center mb-3 text-h3">
            {{ tweenedCurrentVisitors.number.toFixed(0) }}
          </span>
        </v-card>
      </v-col>
      <v-col cols="12" md="8">
        <Line id="my-chart-id" :options="chartOptions" :data="chartData" />
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
import { computed, ref, watch, reactive } from "vue";
import { Line } from "vue-chartjs";
import {
  Chart as ChartJS,
  Title,
  Tooltip,
  Legend,
  LineElement,
  PointElement,
  CategoryScale,
  LinearScale,
} from "chart.js";
import { CronJob } from "cron";
import gsap from "gsap";
import { onMounted } from "vue";

ChartJS.register(
  Title,
  Tooltip,
  Legend,
  PointElement,
  LineElement,
  CategoryScale,
  LinearScale
);

const COLORS = [
  "rgb(255, 99, 132)",
  "rgb(54, 162, 235)",
  "rgb(255, 206, 86)",
  "rgb(75, 192, 192)",
  "rgb(153, 102, 255)",
  "rgb(255, 159, 64)",
  "rgb(255, 99, 132)",
  "rgb(54, 162, 235)",
];

const statDays = ref([
  "Today",
  "Monday",
  "Tuesday",
  "Wednesday",
  "Thursday",
  "Friday",
  "Saturday",
  "Sunday",
]);

const options = { weekday: "long" };
const todayWeekday = new Intl.DateTimeFormat("en-US", options).format(
  new Date()
);

const isUpdatingVisitors = ref(false);
const currentVisitors = ref(0);
const tweenedCurrentVisitors = reactive({
  number: 0,
});

const selectedDays = ref(["Today", todayWeekday]);
const labels = ref([]);
const statsData = ref({});
const filterFrom = ref(getDefaultFilterFrom());
const filterUntil = ref("");
const chartOptions = ref({
  maintainAspectRatio: false,
  aspectRatio: 0.5,
});

const timePickerFrom = ref(false);
const timePickerUntil = ref(false);

const datePicker = ref(false);
const selectedDate = ref(null);

const allowedMinutes = (m) => m % 15 == 0;

function selectDate(e) {
  if (e) {
    selectedDate.value = new Date(e.getTime() - (e.getTimezoneOffset() * 60000 )).toISOString().split('T')[0];
  } else {
    selectedDate.value = null;
  }
  datePicker.value = false;
}

const job = CronJob.from({
  cronTime: "5 */15 * * * *",
  onTick: () =>
    fetchDataAndUpdateStats(
      selectedDays.value,
      filterFrom.value,
      filterUntil.value,
      selectedDate.value
    ),
  start: true,
  timeZone: "Europe/Berlin",
});

// Reset chart data
function resetChartData() {
  labels.value = [];
  statsData.value = {};
}

// Fetch data for selected days and update stats
async function fetchDataAndUpdateStats(selectedDays, filterFrom, filterUntil, selectedDate) {
  resetChartData();
  const promises = selectedDays.map(async (day, index) => {
    const response = await fetch(`/api/${day.toLowerCase()}`);
    const data = await response.json();
    return {
      data,
      day,
      index,
    };
  });
  if (selectedDate) {
    const promise = (async () => {
      const response = await fetch(`/api/date/${selectedDate}`);
      const data = await response.json();
      return {
        data,
        day: selectedDate,
        index: selectedDays.length,
      };
    })();
    promises.push(promise);
  }
  const allData = await Promise.all(promises);
  allData.map(({ data, day, index }) => {
    updateStatsForDay(data, day, index, filterFrom, filterUntil);
  });
}

// Update stats for a specific day
function updateStatsForDay(data, day, index, filterFrom, filterUntil) {
  data.forEach(({ timestamp, value }) => {
    const time = timestamp.split(" ")[1];
    if (filterFrom && time < filterFrom) return;
    if (filterUntil && time > filterUntil) return;
    if (index === 0) labels.value.push(time);
    if (!statsData.value[day]) statsData.value[day] = [];
    statsData.value[day].push(value);
  });
}

function getDefaultFilterFrom(diffHours = 5) {
  const hours = new Date().getHours();
  if (hours > diffHours) {
    const predefiniedHour = (hours - diffHours).toString().padStart(2, 0);
    return `${predefiniedHour}:00`;
  }
  return "";
}

async function refreshCurrentVisitors() {
  try {
    isUpdatingVisitors.value = true;
    currentVisitors.value = 0;
    const response = await fetch(`/api/live`);
    const data = await response.json();
    currentVisitors.value = data.value;
  } catch (err) {
    console.error(err);
  }

  isUpdatingVisitors.value = false;
}

const chartData = computed(() => ({
  labels: labels.value,
  datasets: Object.keys(statsData.value).map((day, index) => ({
    data: statsData.value[day],
    label: day,
    backgroundColor: COLORS[index],
    borderColor: COLORS[index],
    tension: 0.3,
    radius: 5,
    pointHitRadius: 30,
  })),
}));

watch(
  [selectedDays, filterFrom, filterUntil, selectedDate],
  async ([newSelectedDays, newFilterFrom, newFilterUntil, newSelectedDate]) => {
    await fetchDataAndUpdateStats(
      newSelectedDays,
      newFilterFrom,
      newFilterUntil,
      newSelectedDate
    );
  },
  { immediate: true }
);

watch(currentVisitors, (n) => {
  gsap.to(tweenedCurrentVisitors, { duration: 0.5, number: Number(n) || 0 });
});

onMounted(() => {
  refreshCurrentVisitors();
});
</script>
