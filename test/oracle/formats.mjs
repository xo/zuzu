// formats.mjs writes the outputs of d3-format and d3-time-format that the
// tests of the package format compare with. Run it from the test directory:
//
//	TZ=America/New_York node oracle/formats.mjs ../format/testdata
//
// The time zone matters: the local formats read it, and the tests set the
// same zone.
import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { format } from 'd3-format';
import { timeFormat, utcFormat, timeParse, utcParse } from 'd3-time-format';
import { scaleLinear } from 'd3-scale';
import { tickStep } from 'd3-array';

const out = process.argv[2];
if (!out) {
  console.error('usage: node oracle/formats.mjs <dir>');
  process.exit(2);
}

const specs = [
  ',.2f', '~s', '.0%', 'd', '$,.2f', '.3s', 'e', '.2e', '.3g', '.1f', '+.1f',
  '(,.2f', '08.2f', '^10d', '>10,d', '<10d', '*^12,.1f', '012,.2f', '.2r', 'x',
  '#x', 'X', 'o', 'b', '.0f', 'c', '~g', '', '.2~f', '.2p', '~r', '.2~e', '~%',
  ',', ',.0f', '.1s', '~f', '.6~s', '$.3~s', ' d', '-.2f', '.4n', 'n', '.2g',
  '.10f', '.20f', '+$,.2f', '.1%', '=+10.2f',
];
const values = [
  0, -0, 1, -1, 0.125, 2.5, 1234.5678, -0.000123, 1e21, 123456789, 0.5,
  -1234.5, 999.995, 1.005, 42, 0.1 + 0.2, 1e-7, 3.14159265358979, 99999.5,
  1e15, -987654.321, 0.000001, 7e-30, 65,
];
const numbers = [];
for (const s of specs) {
  const f = format(s);
  for (const v of values) numbers.push([s, Object.is(v, -0) ? '-0' : v, f(v)]);
}
writeFileSync(join(out, 'number.json'), JSON.stringify(numbers, null, 0) + '\n');

const directives = [
  '%a', '%A', '%b', '%B', '%c', '%d', '%e', '%f', '%g', '%G', '%H', '%I', '%j',
  '%L', '%m', '%M', '%p', '%q', '%Q', '%s', '%S', '%u', '%U', '%V', '%w', '%W',
  '%x', '%X', '%y', '%Y', '%Z', '%%', '%-d', '%_d', '%0e', '%-I:%M %p',
  '%Y-%m-%dT%H:%M:%S.%LZ', '%b %d, %Y', '%B %-d', '%-m/%-d/%y', '%H:%M',
];
const dates = [
  Date.UTC(2000, 0, 1), Date.UTC(1999, 11, 31, 23, 59, 59, 999),
  Date.UTC(2024, 1, 29, 12, 30, 5, 7), Date.UTC(2021, 0, 3, 7), Date.UTC(2020, 11, 31, 18),
  Date.UTC(1970, 0, 1), Date.UTC(2026, 10, 10, 9, 8, 7, 654), Date.UTC(2026, 2, 8, 6, 30),
  Date.UTC(1965, 6, 4, 15), Date.UTC(2009, 11, 28),
];
const times = [];
for (const d of directives) {
  const local = timeFormat(d);
  const utc = utcFormat(d);
  for (const t of dates) times.push([d, t, local(new Date(t)), utc(new Date(t))]);
}
writeFileSync(join(out, 'time.json'), JSON.stringify(times, null, 0) + '\n');

const parses = [
  ['%Y-%m-%d', '2024-02-29'], ['%Y-%m-%d', '2024-13-01'], ['%m/%d/%Y', '7/4/1965'],
  ['%B %d, %Y', 'November 10, 2026'], ['%b %Y', 'Jan 2000'], ['%Y', '1999'],
  ['%H:%M', '13:45'], ['%I:%M %p', '01:45 PM'], ['%Y-%m-%dT%H:%M:%S.%LZ', '2024-02-29T12:30:05.007Z'],
  ['%Y-%m-%dT%H:%M:%S%Z', '2024-02-29T12:30:05+0530'], ['%d %b %Y', '03 Jan 2021'],
  ['%Q', '1700000000000'], ['%s', '1700000000'], ['%y', '68'], ['%y', '69'],
  ['%Y %U', '2021 05'], ['%Y-%j', '2024-060'], ['%A, %B %e', 'Monday, March  8'],
  ['%Y-%m-%d', 'nope'], ['%x', '7/4/1965'], ['%X', '1:02:03 PM'],
];
const parsed = parses.map(([f, s]) => {
  const a = timeParse(f)(s);
  const b = utcParse(f)(s);
  return [f, s, a ? a.getTime() : null, b ? b.getTime() : null];
});
writeFileSync(join(out, 'parse.json'), JSON.stringify(parsed, null, 0) + '\n');

// The dates that Date parses with no format, as data and toDate read them.
const texts = [
  '2024-02-29', '2024-02', '2024', '2024-02-29T12:30', '2024-02-29T12:30:05.007',
  '2024-02-29T12:30:05Z', '2024-02-29T12:30:05.1+05:30', '2024-02-29T24:00',
  '+002024-02-29', '2024/02/29', '02/29/2024', '2/9/24', 'Feb 29 2024',
  'February 29, 2024', 'February 29, 2024 1:30 PM', '29 Feb 2024', 'Jan 2000',
  'Thu Feb 29 2024 12:30:05 GMT+0530', 'Thu Feb 29 2024 12:30:05 GMT-0500 (Eastern Standard Time)',
  '2024-02-29 12:30', '2024-02-29 12:30:05', '1/1/70', '12/31/49', '12/31/50',
  'Mar 8, 2026 2:30 AM', 'nope', '', '2024-13-01', '2024-02-30', 'Jun 1 1999 23:59:59',
  '1999-06-01T23:59:59.999Z', '2001-01-01T00:00:00-08:00',
];
const dateParse = texts.map((s) => {
  const t = Date.parse(s);
  return [s, Number.isNaN(t) ? null : t];
});
writeFileSync(join(out, 'date.json'), JSON.stringify(dateParse, null, 0) + '\n');

// The labels of the ticks of a linear scale, which an axis or a legend
// writes with tickFormat of d3-scale: a domain, a count, a spec, the step
// of the ticks, the ticks and their labels.
const tickCases = [
  [[0, 25], 5, 's'], [[0, 12e6], 10, 's'], [[0, 100], 10, 's'], [[0, 1.2], 10, 's'],
  [[0, 10], 5, '~s'], [[-3e3, 4e3], 7, '.1s'], [[0, 1], 5, ''], [[0, 1.1], 10, ''],
  [[0, 2.5e9], 5, ''], [[0, 1], 10, '%'], [[0, 0.05], 5, '%'], [[0, 1], 4, 'e'],
  [[1, 1.1], 10, 'r'], [[0, 1e-3], 5, 'g'], [[0, 50], 10, ','], [[0, 0.3], 6, ',f'],
  [[100, 200], 5, '$'], [[0, 5000], 5, ',.2f'], [[-1, 1], 8, '+f'], [[0, 1], 5, 'p'],
];
const ticks = tickCases.map(([d, count, spec]) => {
  const s = scaleLinear().domain(d);
  const f = s.tickFormat(count, spec || undefined);
  const t = s.ticks(count);
  return [spec, d[0], d[1], Math.abs(tickStep(d[0], d[1], count)), t, t.map(f)];
});
writeFileSync(join(out, 'ticks.json'), JSON.stringify(ticks, null, 0) + '\n');
