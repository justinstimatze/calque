package code

import "sort"

// WeightSample is one adjudicated suspect: the per-channel signal values that
// scorePair computed for the pair, plus whether the registry labeled it a real
// find (drift / clone-worthy) or noise (false-alarm). This is the §13-clean
// training row — a human verdict on a concrete pair, never a self-scan vibe.
type WeightSample struct {
	Signals map[string]float64
	Useful  bool
}

// CalibrationStats records what CalibrateWeights observed, so the caller can
// report it and a reviewer can sanity-check the move before trusting the vector.
type CalibrationStats struct {
	N         int                // total samples
	NUseful   int                // labeled-useful count
	NNotUsefl int                // labeled-not-useful count
	Lambda    float64            // shrinkage weight on the observed signal, n/(n+priorStrength)
	MeanDiff  map[string]float64 // per-channel (mean useful − mean not-useful), the raw discrimination
	Raw       map[string]float64 // normalized observed weights before shrinkage
}

// CalibrateWeights derives a weight vector from adjudicated samples by measuring
// how well each channel SEPARATES useful from not-useful pairs, then shrinking
// that observation toward the static prior so a handful of adjudications can't
// overfit.
//
// Per channel: discrimination = mean(signal | useful) − mean(signal | not-useful).
// A channel that reads higher on real finds than on noise earns weight; one that
// doesn't (≤0 difference) is floored to 0 observed weight (the prior still carries
// it via shrinkage). The positive differences are normalized to sum to 1 → the
// observed weights. The result is lambda*observed + (1−lambda)*prior, with
// lambda = n/(n+priorStrength): few samples ⇒ stay near the prior, many ⇒ trust
// the data. Both label classes must be present (caller's responsibility; with one
// class every mean-difference is undefined and this returns the prior unchanged).
//
// hasAnchor in scorePair is weight-independent, so a calibrated vector only
// re-ranks anchored pairs; it can never surface or suppress an anchor. The output
// is always normalized over channelOrder and sums to 1.
func CalibrateWeights(samples []WeightSample, prior map[string]float64, priorStrength float64) (map[string]float64, CalibrationStats) {
	if prior == nil {
		prior = weights
	}
	prior = normalizeWeights(cloneWeights(prior))

	stats := CalibrationStats{
		Lambda:   0,
		MeanDiff: map[string]float64{},
		Raw:      cloneWeights(prior),
	}

	// Partition sums by label, per channel.
	sumUseful := map[string]float64{}
	sumNot := map[string]float64{}
	var nUseful, nNot int
	for _, s := range samples {
		if s.Useful {
			nUseful++
			for _, k := range channelOrder {
				sumUseful[k] += s.Signals[k]
			}
		} else {
			nNot++
			for _, k := range channelOrder {
				sumNot[k] += s.Signals[k]
			}
		}
	}
	stats.N = len(samples)
	stats.NUseful = nUseful
	stats.NNotUsefl = nNot

	// Single-class (or empty): no discrimination signal — return the prior.
	if nUseful == 0 || nNot == 0 {
		for _, k := range channelOrder {
			stats.MeanDiff[k] = 0
		}
		return cloneWeights(prior), stats
	}

	// Per-channel discrimination, floored at 0 (a channel that reads no higher on
	// real finds earns no observed weight).
	pos := map[string]float64{}
	var posSum float64
	for _, k := range channelOrder {
		diff := sumUseful[k]/float64(nUseful) - sumNot[k]/float64(nNot)
		stats.MeanDiff[k] = diff
		if diff > 0 {
			pos[k] = diff
			posSum += diff
		}
	}

	// Normalize positive discriminations into observed weights. If NO channel
	// discriminates (posSum==0, e.g. identical distributions), fall back to prior.
	observed := map[string]float64{}
	if posSum == 0 {
		observed = cloneWeights(prior)
	} else {
		for _, k := range channelOrder {
			observed[k] = pos[k] / posSum
		}
	}
	stats.Raw = cloneWeights(observed)

	// Shrink toward prior: lambda = n/(n+priorStrength).
	n := float64(len(samples))
	lambda := n / (n + priorStrength)
	stats.Lambda = lambda
	out := map[string]float64{}
	for _, k := range channelOrder {
		out[k] = lambda*observed[k] + (1-lambda)*prior[k]
	}
	return normalizeWeights(out), stats
}

// normalizeWeights scales a weight map to sum to 1 over channelOrder. A zero-sum map
// (degenerate) is returned as the uniform vector so scorePair never divides by a
// zero wsum on every pair.
func normalizeWeights(w map[string]float64) map[string]float64 {
	var sum float64
	for _, k := range channelOrder {
		sum += w[k]
	}
	out := make(map[string]float64, len(channelOrder))
	if sum == 0 {
		u := 1.0 / float64(len(channelOrder))
		for _, k := range channelOrder {
			out[k] = u
		}
		return out
	}
	for _, k := range channelOrder {
		out[k] = w[k] / sum
	}
	return out
}

// SortedChannels returns channelOrder copied, so callers (reports) can render a
// stable table without reaching into the package's unexported order.
func SortedChannels() []string {
	out := append([]string(nil), channelOrder...)
	sort.Strings(out)
	return out
}
