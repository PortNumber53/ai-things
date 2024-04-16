import { Composition, staticFile } from 'remotion';
import { AudioGramSchema, AudiogramComposition, fps } from './Composition';
import './style.css';

export const RemotionRoot: React.FC = () => {
	return (
		<>
			<Composition
				id="Audiogram"
				component={AudiogramComposition}
				fps={fps}
				width={1080}
				height={1920}
				schema={AudioGramSchema}
				defaultProps={{
					// Audio settings
					audioOffsetInSeconds: 0,

					// Title settings
					audioFileName: staticFile('__REPLACE_WITH_MP3__'),
					coverImgFileName: staticFile('__REPLACE_WITH_IMAGE__'),
					titleText: '__REPLACE_WITH_TITLE__',
					titleColor: 'rgba(186, 186, 186, 0.93)',

					// Subtitles settings
					subtitlesFileName: staticFile('__REPLACE_WITH_SUBTITLES__'),
					onlyDisplayCurrentSentence: true,
					subtitlesTextColor: 'rgba(255, 255, 255, 0.93)',
					subtitlesLinePerPage: 8,
					subtitlesZoomMeasurerSize: 4,
					subtitlesLineHeight: 70,

					// Wave settings
					waveColor: '#93d5ae',
					waveFreqRangeStartIndex: 7,
					waveLinesToDisplay: 40,
					waveNumberOfSamples: '256', // This is string for Remotion controls and will be converted to a number
					mirrorWave: false,
					durationInSeconds: __DURATION__,
				}}
				// Determine the length of the video based on the duration of the audio file
				calculateMetadata={({ props }) => {
					return {
						durationInFrames: props.durationInSeconds * fps,
						props,
					};
				}}
			/>
		</>
	);
};
