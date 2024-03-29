<template>
  <v-container class="fill-height pa-4">
    <v-row>
      <v-col cols="12" md="4">
        <v-select label="Days" :items="statDays" v-model="selectedDays" multiple></v-select>
        <v-text-field label="From" v-model="filterFrom"></v-text-field>
        <v-text-field label="Until" v-model="filterUntil"></v-text-field>
      </v-col>
      <v-col cols="12" md="8">
        <Line id="my-chart-id" :options="chartOptions" :data="chartData" />
      </v-col>
    </v-row>
  </v-container>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import { Line } from 'vue-chartjs'
import { Chart as ChartJS, Title, Tooltip, Legend, LineElement, PointElement, CategoryScale, LinearScale } from 'chart.js'

ChartJS.register(Title, Tooltip, Legend, PointElement, LineElement, CategoryScale, LinearScale);

const COLORS = [
  'rgb(255, 99, 132)',
  'rgb(54, 162, 235)',
  'rgb(255, 206, 86)',
  'rgb(75, 192, 192)',
  'rgb(153, 102, 255)',
  'rgb(255, 159, 64)',
  'rgb(255, 99, 132)',
  'rgb(54, 162, 235)'
];

const statDays = ref(["Today", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"]);
const selectedDays = ref(["Today"]);
const labels = ref([]);
const statsData = ref({});
const filterFrom = ref("");
const filterUntil = ref("");
const chartOptions = ref({
  maintainAspectRatio: false,
  aspectRatio: 0.5,
});


watch(
  [selectedDays, filterFrom, filterUntil],
  async ([newSelectedDays, newFilterFrom, newFilterUntil]) => {
    resetChartData();
    await fetchDataAndUpdateStats(newSelectedDays, newFilterFrom, newFilterUntil);
  },
  { immediate: true }
);

// Reset chart data
function resetChartData() {
  labels.value = [];
  statsData.value = {};
}

// Fetch data for selected days and update stats
async function fetchDataAndUpdateStats(selectedDays, filterFrom, filterUntil) {
  const promises = selectedDays.map(async (day, index) => {
    const response = await fetch(`/api/${day.toLowerCase()}`);
    const data = await response.json();
    updateStatsForDay(data, day, index, filterFrom, filterUntil);
  });
  await Promise.all(promises);
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
  }))
}));

</script>