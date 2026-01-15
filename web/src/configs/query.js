export const suggestions = [
  {
    value: `SELECT * FROM shadow`,
    label: "things.querySuggestions.selectAll",
    autoTrigger: true,
  },
  {
    value: `SELECT * FROM shadow ORDER BY createdAt`,
    label: "things.querySuggestions.sortByCreated",
    autoTrigger: true,
  },
  {
    value: `SELECT * FROM shadow ORDER BY updatedAt DESC`,
    label: "things.querySuggestions.sortByUpdatedDesc",
    autoTrigger: true,
  },
  // {
  //   value: `SELECT * FROM shadow ORDER BY updatedAt DESC LIMIT 1`,
  //   label: "things.querySuggestions.selectLastUpdated",
  //   autoTrigger: true,
  // },
  {
    value: `SELECT thingId, connected, \`state.reported\` as reported, \`state.desired\` as desired, updatedAt FROM shadow`,
    label: "things.querySuggestions.selectStatus",
    autoTrigger: true,
  },
  {
    value: `SELECT thingId, connected, \`state.reported\`, createdAt as created_time, updatedAt as updated_time, \`tags\` FROM shadow`,
    label: "things.querySuggestions.renameTimeFields",
    autoTrigger: true,
  },
  {
    value: `SELECT * FROM shadow WHERE connected = true`,
    label: "things.querySuggestions.allConnected",
    autoTrigger: true,
  },
  {
    value: `SELECT * FROM shadow
         WHERE connected = false
         OR connected IS NULL`,
    label: "things.querySuggestions.disconnected",
    autoTrigger: true,
  },
  {
    value: "SELECT * FROM shadow WHERE thingId = 'your-thingId'",
    label: "things.querySuggestions.selectSpecified",
    autoTrigger: false,
  },
  {
    value: "SELECT * FROM shadow WHERE createdAt > '2026-01-01 00:00:00'",
    label: "things.querySuggestions.byCreatedTime",
    autoTrigger: false,
  },
  {
    value: "SELECT * FROM shadow WHERE `tags.yourTagName` = 'yourTagValue'",
    label: "things.querySuggestions.byTagName",
    autoTrigger: false,
  },
  {
    value:
      "SELECT * FROM shadow WHERE `state.desired.yourPropName` = 'yourPropValue'",
    label: "things.querySuggestions.byDesiredProp",
    autoTrigger: false,
  },
  {
    value:
      "SELECT * FROM shadow WHERE `state.reported.yourPropName` = 'yourPropValue'",
    label: "things.querySuggestions.byReportedProp",
    autoTrigger: false,
  },
];
