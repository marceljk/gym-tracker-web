const axios = require('axios');
const sqlite3 = require('sqlite3').verbose();
const cron = require('node-cron');
const moment = require('moment');

const express = require('express');
const app = express();
const PORT = process.env.PORT || 3030;
const STATS_WEEKS = process.env.STATS_WEEKS || 8;

const API_URL = process.env.API_URL;

const API_OPTIONS = {
  headers: {
    "X-Tenant": process.env.X_Tenant
  }
};

const db = new sqlite3.Database('./db/data.db');

db.serialize(() => {
  db.run('CREATE TABLE IF NOT EXISTS stats (timestamp TEXT PRIMARY KEY, value REAL, weight INTEGER)');
  db.run('CREATE TABLE IF NOT EXISTS history (timestamp TEXT PRIMARY KEY, value REAL)');
});

// Request every 15 minutes and insert to db
cron.schedule('*/15 * * * *', async () => {
  try {
    const response = await axios.get(API_URL, API_OPTIONS);
    const { value } = response.data;

    const timestamp = moment().format("dddd, HH:mm")

    const existingRecord = await new Promise((resolve, reject) => {
      db.get('SELECT value, weight FROM stats WHERE timestamp = ?', [timestamp], (err, res) => {
        if (err) {
          console.error("Failed get stats from db", err);
          reject(err);
        }
        resolve(res);
      });
    });

    if (existingRecord) {
      const { value: existingValue, weight: existingWeight } = existingRecord;
      const newWeight = existingWeight + 1;
      const averageValue = (existingValue * existingWeight + value) / newWeight;
      await db.run('UPDATE stats SET value = ?, weight = ? WHERE timestamp = ?', [averageValue, newWeight, timestamp]);
    } else {
      await db.run('INSERT INTO stats (timestamp, value, weight) VALUES (?, ?, ?)', [timestamp, value, 1]);
    }
    await db.run('INSERT INTO history (timestamp, value) VALUES (?, ?)', [moment().format("YYYY/MM/DD HH:mm"), value]);

    console.log('Data stored successfully for timestamp:', timestamp);
  } catch (error) {
    console.error('Error occurred:', error);
  }
});

const requestDataFromDB = async (day) => {
  let sql = "SELECT * FROM stats WHERE timestamp LIKE ? ORDER BY timestamp";
  let params = [`${day}%`];

  if (day === "today") {
    const todaysDate = moment().format("YYYY/MM/DD");
    sql = "SELECT * FROM history WHERE timestamp LIKE ? ORDER BY timestamp";
    params = [`${todaysDate}%`];

    const results = await new Promise((resolve, reject) => {
      db.all(sql, params, (err, res) => {
        if (err) {
          console.error("Failed to get stats from db", err);
          reject(err);
        }
        resolve(res);
      });
    });
    return results;
  } else {
    return getAverageOfLastXWeeks(day)
  }
}

const getLastWeekdayDate = (weekdayName) => {
  const targetDay = weekdays.indexOf(weekdayName);

  if (targetDay === -1) {
    throw new Error("Invalid weekday name. Use full names like 'monday', 'tuesday', etc.");
  }

  const today = new Date();
  const todayDay = today.getUTCDay();

  let difference = targetDay - todayDay;
  if (difference >= 0) {
    difference -= 7; // Move to the previous week
  }

  const lastDate = new Date(today);
  lastDate.setDate(today.getDate() + difference);

  return lastDate;
}

const getAverageOfLastXWeeks = async (day, weeks = STATS_WEEKS) => {
  if (!weekdays.includes(day)) return;
  let result = {}

  let date = getLastWeekdayDate(day)
  for (let i = 0; i < weeks; i++) {
    const timestamp = new Date(date)
    timestamp.setDate(timestamp.getDate() - (7 * i))

    const dateString = moment(timestamp).format("YYYY/MM/DD");
    sql = "SELECT * FROM history WHERE timestamp LIKE ? ORDER BY timestamp";
    params = [`${dateString}%`];

    const request = new Promise((resolve, reject) => {
      db.all(sql, params, (err, res) => {
        if (err) {
          console.error("Failed to get stats from db", err);
          reject(err);
        }
        resolve(res);
      });
    });

    dbResult = await Promise.resolve(request)
    dbResult.forEach((entry) => {
      let day = moment(entry.timestamp, "YYYY/MM/DD HH:mm").format("dddd, HH:mm")
      if (day in result) {
        record = result[day]
        const newWeight = record.weight + 1
        const newValue = ((record.value * record.weight) + entry.value) / newWeight
        result[day] = {
          weight: newWeight,
          value: newValue
        }
      } else {
        result[day] = {
          weight: 1,
          value: entry.value
        }
      }
    })
  }
  return Object.entries(result).map(([timestamp, data]) => ({ timestamp, value: data.value }))
}

const weekdays = moment.weekdays().map((weekday) => {
  return weekday.toLowerCase();
});

const routes = [...weekdays.map((day) => "/" + day), "/today"];

app.get(routes, async (req, res) => {
  const day = req.path.replace("/", "");
  const results = (await requestDataFromDB(day)).map(({ timestamp, value }) => ({ timestamp, value }));
  res.json(results);
})

app.get("/live", async (req, res) => {
  try {
    const response = await axios.get(API_URL, API_OPTIONS);
    res.json(response.data);
  } catch (err) {
    console.error(err)
    res.status(424).json({ err });
  }
})

app.listen(PORT, () => {
  console.log(`Using ${STATS_WEEKS} weeks for stats`)
  console.log(`GYM Tracker listening at http://localhost:${PORT}`);
});