function updateWatchStatus(imdb_code, state, progress) {
	window.parent.postMessage(JSON.stringify({
		"type": "update_watch_status",
		"data": {
			imdb_code: imdb_code,
			state: state,
			progress: progress
		}
	}), "*");
}

$(function(){
	// Handle video event.
	var renderVideo = (event, video) => {
		console.log("received event!", video);

		// Retrieve the URL of the stream.
		var stream_url = video.url;
		var on = video.item;
		var filename = stream_url.slice(0, stream_url.lastIndexOf('?'));

		// Initialize player markup.
		$('#player').empty();
		var elem = $('<video id="video-player" class="video-js vjs-default-skin" controls preload="auto"></video>');
		var mime_type_guess = 'video/' + filename.split('.').slice(-1)[0]; // does not work for .ogv videos, among others
		if(filename.endsWith('.mkv')){
			mime_type_guess = "video/webm"; // workaround to allow .mkv files to be played
		}
		elem.append('<source src="' + stream_url + '" type="' + mime_type_guess + '"></source>');
		$('#player').append(elem);

		// Implement player auto-resizing.
		var player = null;
		var resizePlayer = () => {
			setTimeout(() => {
				const size = [$(window).width(), $(window).height()];
				console.log("Resizing player:", size);
				player.width(size[0]);
				player.height(size[1]);
			}, 500);
		};

		// Define helper function for video stop
		var onPlaybackStopped = () => {
			if(on){
				var progress = (100.0 * player.currentTime()) / (player.duration());
				if(progress < 90.0){
					updateWatchStatus(on.imdb_code, "paused", progress);
				} else {
					console.log("Assuming user finished watching movie.");
					updateWatchStatus(on.imdb_code, "stopped", progress); // if progress above 80%, Trakt assumes you're done watching movie
				}
			}
		};

		// Install AirPlay button if video format is supported.
		var vjsButtonComponent = videojs.getComponent('Button');
		videojs.registerComponent('AirplayButton', videojs.extend(vjsButtonComponent, {
			constructor: function () {
				vjsButtonComponent.apply(this, arguments);
			},
			handleClick: function () {
				console.log("Airplay requested.");
				// TODO: startAirplayPlayback with `url`, `progress`
				window.parent.postMessage(JSON.stringify({
					"type": "start_airplay",
					"data": {
						url: video.url,
						progress: player.currentTime() / player.duration()
					}
				}), "*");
			},
			buildCSSClass: function () {
				return 'vjs-control vjs-airplay-button';
			},
			createControlTextEl: function (button) {
				return $(button).html($('<i class="material-icons" style="font-size: 1.5em;">airplay</i>').attr('title', 'Airplay'));
			}
		}));

		// Instantiate player.
		player = videojs('video-player', {
			//"techOrder": ["Vlc"]
		}, function onPlayerReady() {
			// Hide loading text.
			$('#loading-text').hide();
			console.log("Video player is ready.");

			// Show AirPlay button.
			this.getChild('controlBar').addChild('AirplayButton', {});

			// Start playing.
			this.play();

			// Show window when playback begins.
			var first_play_start = true;
			var ignore_this_pause = false;
			this.on('playing', function() {
				console.log("Started playing.");
				var should_update_scrobble = true;
				if(first_play_start){
					first_play_start = false;
					resizePlayer();
					var progress = on.playback_progress;
					if(progress){
						ignore_this_pause = true;
						should_update_scrobble = false;
						player.pause();
						swal({
							icon: "info",
							title: "Resume?",
							text: "Do you want to resume from where you left off?",
							buttons: {
								cancel: "No",
								confirm: "Yes!"
							}
						}).then((resp) => {
							if(!resp){
								updateWatchStatus(on.imdb_code, "started", /*progress=*/0.0);
								player.play();
								return;
							}
							console.log("Resuming from scrobble.");
							// var progress = (100.0 * player.currentTime()) / (player.duration());
							// progress = 100 * x / k
							// x = (k * progress) / 100
							var to_seek_to = (player.duration() * progress) / 100.0;
							console.log("Seeking to:", to_seek_to, "seconds");
							player.currentTime(to_seek_to);
							player.play();
							let notif = new Notification('Resumed From Scrobble', {
								body: "Continued from where you left off.",
								icon: on.cover_image,
								silent: true
							});
							notif.onclick = () => {};
						});
					}
				}
				if(on && should_update_scrobble){
					var progress = (100.0 * player.currentTime()) / (player.duration());
					updateWatchStatus(on.imdb_code, "started", progress);
				}
			});
			this.on('pause', function() {
				if(ignore_this_pause){
					ignore_this_pause = false;
					return;
				}
				console.log("Detected video pause");
				if(on){
					var progress = (100.0 * player.currentTime()) / (player.duration());
					updateWatchStatus(on.imdb_code, "paused", progress);
				}
			});
			this.on('ended', function() {
				console.log("Detected video ended");
				onPlaybackStopped();
			});
			this.on('userinactive', function() {
				// Hide cursor.
				$('html').css('cursor', 'none');
			});
			this.on('useractive', function() {
				// Show cursor.
				$('html').css('cursor', 'default');
			});
		});

		window.onbeforeunload = function() {
			console.log("onbeforeunload triggered");
			if(!player.paused()){
				onPlaybackStopped();
			}
		};
	};

	window.addEventListener("message", (e) => {
		setTimeout(() => {
			console.log("Got a message:", e);
			renderVideo(e, JSON.parse(e.data));
		}, 10);
	}, false);

	window.parent.postMessage(JSON.stringify({
		"type": "watch_window_open",
		"data": {}
	}), "*");

	console.log("Initialized player.");
});
