'use strict';

var zlib = require('zlib');
var fs = require('fs');
var express = require('express');
var util = require('util');
var stream = require('stream');

var app = express();

var LOG_PATH = '/var/overrustle/logs/Lirik chatlogs/May 2015';

app.get('/test/:name', function(req, res) {
  var logs = fs.readdir(LOG_PATH, function(err, files) {
    if (err) {
      res.status(500).send(err);
    }
    else {
      enqueueFile();
    }

    function enqueueFile() {
      if (files.length) {
        res.status(200);
        processFile(files.shift(), enqueueFile);
      }
      else {
        res.end();
      }
    }

    function processFile(file, done) {
      var src = fs.readFile(file);

      if (/.gz$/.test(file)) {
        src = src.pipe(zlib.createGunzip());
      }

      src
        .pipe(new Filter(req.params.name))
        .on('data', function(chunk) {
          res.write(chunk);
        })
        .on('end', done);
    }
  });
});

function Filter(name) {
  stream.Transform.call(this);

  this._buffer = '';
  this._name = name;
  this._nameLength = this._name.length;
}

util.inherits(Filter, stream.Transform);

Filter.prototype._test = function(line) {
  return line.substring(28, 28 + this._nameLength) === this._name;
};

Filter.prototype._transform = function(data, encoding, callback) {
  var lines = data.split("\n");

  lines[0] = this._buffer + lines[0];
  this._buffer = lines.pop();

  for (var i = 0; i < lines.length; i ++) {
    if (this._test(lines[i])) {
      this.push(lines[i] + "\n");
    }
  }

  callback();
};

Filter.prototype._flush = function(callback) {
  if (this._test(this._buffer)) {
    this.push(this._buffer);
  }

  callback();
};

app.listen(8080);