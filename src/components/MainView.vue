<template>
  <v-container class="fill-height pa-4">
    <v-row>
      <v-col cols="12" md="4">
        <v-select label="Day" :items="statDays" v-model="selectedDay"></v-select>
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

const statDays = ref(["Today", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"]);
const selectedDay = ref("Today");
const labels = ref([]);
const statsData = ref([]);

const filterFrom = ref("");
const filterUntil = ref("");

watch(
  [selectedDay, filterFrom, filterUntil],
  async ([newSelectedDay, newFilterFrom, newFilterUntil]) => {
    const response = await fetch("/api/" + newSelectedDay.toLowerCase());
    const data = await response.json();
    labels.value = [];
    statsData.value = [];
    data.forEach(({ timestamp, value }) => {
      const time = timestamp.split(" ")[1];
      if (newFilterFrom !== "" && time < newFilterFrom) return;
      if (newFilterUntil !== "" && time > newFilterUntil) return;
      labels.value.push(time);
      statsData.value.push(value);
    });
  },
  { immediate: true }
);


const chartData = computed(() => ({
  labels: labels.value,
  datasets: [{
    data: statsData.value,
    label: selectedDay.value,
    backgroundColor: 'rgb(75, 192, 192)',
    borderColor: 'rgb(75, 192, 192)',
    tension: 0.3,
    radius: 5,
    pointHitRadius: 30,
  }]
}))

const chartOptions = ref({
  maintainAspectRatio: false,
  aspectRatio: 0.5,
})
</script>